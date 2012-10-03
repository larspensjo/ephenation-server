// Copyright 2012 The Ephenation Authors
//
// This file is part of Ephenation.
//
// Ephenation is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// Ephenation is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Ephenation.  If not, see <http://www.gnu.org/licenses/>.
//

package main

import (
	"bytes"
	"chunkdb"
	"client_prot"
	cryptrand "crypto/rand"
	"crypto/rc4"
	sync "evalsync"
	"fmt"
	"github.com/robfig/goconfig/config"
	"io/ioutil"
	"license"
	"log"
	"math"
	"math/rand"
	"net"
	"quadtree"
	"score"
	"strings"
	"time"
	. "twof"
)

//
// This is the manager of users. All functions in this file are called from the client process only.
//

// Constants use for the user 'connState'.
const (
	PlayerConnStateLogin = iota // The player has to login
	PlayerConnStatePass  = iota // The player has to provide a password
	PlayerConnStateIn    = iota // The player is in the world
	PlayerConnStateDisc  = iota // The player is logged in but disconnected
)

// This is a command sent from any process to the client process. The purpose is to minimize access the
// user structure from other threads.
type ClientCommand func(up *user)

// The user struct contains information about the player, as represented in the server. Only the "pl" part is
// saved when the player logs out.
type user struct {
	conn                       net.Conn          // The TCP/IP connectin to the player.
	pl                         player            // This is what is saved. All other data is volatile.
	mvFwd, mvBwd, mvLft, mvRgt bool              // Flags if player is moving forward, backward, strafing left or strafing right, can change asynchronously anytime.
	updatedStats               bool              // The player has an updated HP/Level/Exp that must be communicated to the client.
	forceSave                  bool              // Save the player next possible opportunity
	startMoving                time.Time         // Time stamp when player position was last updated
	prevCoord                  user_coord        // The previous player coordinates
	connState                  uint8             // The player connection state. See definition of PlayerConnState*
	objMoved                   []quadtree.Object // This is a list of near objects that moved recently. Used for reporting movements to the client
	key                        []byte            // Used for the decryption
	uid                        uint32            // A unique id, needed by the client to identify players.
	sync.RWMutex                                 // Used for read and write locking a user
	fullLicense                bool              // True if this is a logged in using the full paying license (#2).
	challenge                  []byte            // Used at login, and then again to verify the password.
	lic                        *license.License  // The license associated with this player
	channel                    chan []byte       // Data to be sent to the client is only handled by the listener process, all else must go through this channel. See writeNonBlocking()
	commandChannel             chan ClientCommand
	aggro                      *monster // The monster we are attacking, if any
	flags                      uint32   // Bit mapped flags that the client always have to know about. See UserFlag* in client_prot.
	// Data for trap management
	trapPrevBlock block // The previous block type. A trap shall trig only when going into it from outside
}

func (this *user) String() string {
	return fmt.Sprintf("{user %#v move:(%v %v %v %v) prev:%v state:%v id:%v fullLicense:%v, lic:%v}",
		this.pl.String(), this.mvFwd, this.mvBwd, this.mvLft, this.mvRgt, this.prevCoord, this.connState, this.uid, this.fullLicense, this.lic)
}

var (
	allPlayersSem    sync.RWMutex             // Used to synchronize access to all data structures in this var group
	allPlayers       [MAX_PLAYERS]*user       // This array contains all players
	lastPlayerSlot   int                      // The last slot in use in allPlayers
	numPlayers       int                      // The total number of players currently
	allPlayerNameMap = make(map[string]*user) // Map from player name to user
	allPlayerIdMap   = make(map[uint32]*user) // Map from player id to user
)

var (
	// All players and monsters are stored in an Quadtree, to make it quick to find distances.
	lowerLeftNearCorner = TwoF{-QuadtreeInitSize, -QuadtreeInitSize}
	upperLeftFarCorner  = TwoF{QuadtreeInitSize, QuadtreeInitSize}
	playerQuadtree      = quadtree.MakeQuadtree(lowerLeftNearCorner, upperLeftFarCorner, 1)
)

// A new connection
func NewClientConnection_WLa(conn net.Conn) (ok bool, index int) {
	allPlayersSem.Lock()
	defer allPlayersSem.Unlock()
	// Find an empty slot for the new player. TODO: More efficient algorithm could be used.
	var i int
	for i = 0; i < MAX_PLAYERS; i++ {
		if allPlayers[i] == nil {
			break
		}
	}
	if i == MAX_PLAYERS {
		// TODO: Handle the case with too many players
		return false, 0
	}
	up := new(user)
	// No need to lock this specific player as the 'open' flag is the last to enable.
	// And players can only be removed or added if the 'allPlayersSem' isn't locked.
	up.conn = conn
	up.connState = PlayerConnStateLogin
	up.startMoving = time.Now()
	up.objMoved = make([]quadtree.Object, 0, 10) // length 0, reserve 10 elements.
	up.channel = make(chan []byte, ClientChannelSize)
	up.commandChannel = make(chan ClientCommand, ClientChannelSize)
	// log.Printf("ClientConnection: new player for slot %d\n", i)
	if i >= lastPlayerSlot {
		lastPlayerSlot = i + 1
	}
	numPlayers++
	allPlayers[i] = up
	return true, i
}

// The 'name' argument is not the nick name shown, it is the login name used to
// authenticate the player.
func (up *user) CmdLogin_WLwWLuWLqBlWLc(email string) {
	// fmt.Printf("CmdLogin: New player %v\n", email)
	// It may be that there is no license for this player. But we can only give one type of error
	// message, which means wrong email or password.
	validTestUser := false
	remote := up.conn.RemoteAddr().String()
	addr := strings.Split(remote, ":") // The format is expected to be NNN.NNN.NNN.NNN:NNNN.
	if *allowTestUser && strings.HasPrefix(email, CnfgTestPlayerNamePrefix) {
		if len(addr) == 2 {
			ip := addr[0]
			cnfg, err := config.ReadDefault(*configFileName)
			if err == nil && cnfg.HasSection("login") {
				testplayersallowed, _ := cnfg.Bool("login", "testplayer")
				testiplist, err := cnfg.String("login", "testip")
				if testplayersallowed && err == nil && strings.Contains(testiplist, ip) {
					validTestUser = true
				} else if err != nil {
					validTestUser = true // Allow testuser if no "testip" key.
				}
			} else {
				validTestUser = true // Allow testuser if no config file or no "Login" section
			}
		}
	} else {
		log.Println("Denied testuser from", remote)
	}
	if validTestUser {
		// This test player is allowed login without password, but it is never saved
		uid := up.pl.New_WLwWLc(email)
		up.uid = uid
		up.loginAck_WLuWLqBlWLa()
		up.pl.adminLevel = 9
	} else {
		lic := license.Load_Bl(email)
		up.Lock()
		up.lic = lic
		up.challenge = make([]byte, LoginChallengeLength)
		cryptrand.Read(up.challenge)
		if lic != nil && len(lic.Names) > 0 {
			uid, ok := up.pl.Load_WLwBlWLc(lic.Names[0])
			if ok {
				up.uid = uid
			} else {
				up.lic = nil // Failed to load player data, have fail login (do not know avatar name)
			}
		} else if lic == nil {
			if *verboseFlag > 0 {
				log.Printf("Login failed or no license for '%v'\n", email)
			}
		} else if len(lic.Names) == 0 {
			log.Printf("No avatar for email '%v'\n", email)
			up.lic = nil // Force the next login phase to fail.
		}
		up.connState = PlayerConnStatePass
		up.Unlock()
		// Request a password, even though the license may be incorrect.
		up.writeBlocking_Bl([]byte{3 + LoginChallengeLength, 0, client_prot.CMD_REQ_PASSWORD})
		up.writeBlocking_Bl(up.challenge)
	}
}

// Create a new vector where all numbers are the XOR values from the two
// input vectors. If one is shorter than the other, then simply fill up
// the last numbers.
func xorVector(v1, v2 []byte) (ret []byte) {
	l1, l2 := len(v1), len(v2)
	max := l1
	if l2 > l1 {
		max = l2
	}
	ret = make([]byte, max)
	for i := 0; i < max; i++ {
		if i >= l1 {
			ret[i] = v2[i]
		} else if i >= l2 {
			ret[i] = v1[i]
		} else {
			ret[i] = v1[i] ^ v2[i]
		}
	}
	return
}

// Check the password of the player.
// Return false if connection shall be disonnected
func (up *user) CmdPassword_WLwWLuWLqBlWLc(encrPass []byte) bool {
	// The password is given by the client as an encrypted byte vector.
	// fmt.Printf("CmdPassword: New player encr passwd%v\n", encrPass)
	if up.lic == nil {
		// CmdLogin_WLwWLuWLqBlWLc(up.pl.name, index)
		if *verboseFlag > 0 {
			log.Println("Terminate because of no license")
		}
		return false
	}
	// Decrypt the password using the full license key.
	cipher, err := rc4.NewCipher(xorVector([]byte(up.lic.License), up.challenge))
	if err != nil {
		log.Printf("CmdPassword: NewCipher1 returned %v\n", err)
		return false
	}
	passw := make([]byte, len(encrPass))
	cipher.XORKeyStream(passw, encrPass)
	// fmt.Printf("CmdPassword: Decrypted password is %#v\n", string(passw))
	if !up.lic.VerifyPassword(string(passw), encryptionSalt) {
		// fmt.Println("CmdPassword: stored password doesn't match the given")
		// CmdLogin_WLwWLuWLqBlWLc(up.pl.name, index)
		if *verboseFlag > 0 {
			log.Println("Terminate because of bad password")
		}
		return false
	}
	// Save player logon time
	up.lic.SaveLogonTime_Bl()
	up.loginAck_WLuWLqBlWLa()
	return true
}

// A user has been accepted as a player. Send ack and inform near objects
func (up *user) loginAck_WLuWLqBlWLa() {
	up.ReportAllInventory_WluBl()
	// Don't need lock yet, as the used data until now is constant.
	const msgLen = 12
	var b [msgLen]byte
	b[0] = msgLen
	b[1] = 0
	b[2] = client_prot.CMD_LOGIN_ACK
	EncodeUint32(up.uid, b[3:7])
	EncodeUint16(uint16(up.pl.dirHor*100), b[7:9])
	EncodeUint16(uint16(up.pl.dirVert*100), b[9:11])
	b[11] = up.pl.adminLevel
	up.writeBlocking_Bl(b[:])
	up.prevCoord = up.pl.coord
	// Find all near players and tell them
	near := playerQuadtree.FindNearObjects_RLq(up.GetPreviousPos(), client_prot.NEAR_OBJECTS)
	// fmt.Printf("Near objects to new player at %v: %v\n", up.pl.coord, near)

	// Iterate over all near players. We will not find self, as 'up' is not in the quadtree yet.
	for _, o := range near {
		other, ok := o.(*user)
		if !ok {
			log.Println("Not only players in player quad tree")
			continue // Only need to tell players, not monsters etc.
		}
		other.ReportEquipment_Bl(up)
		// Tell 'other' that we moved
		other.Lock()
		other.SomeoneMoved(up)
		other.Unlock()
		// Tell the new player that the 'other' moved. No lock is needed in this case.
		up.SomeoneMoved(other)
	}
	up.ReportEquipment_Bl(up) // Should also go to self

	// Add self after iterations over near players. Until now, no lock has been needed for 'up' as
	// player hasn't been available in the quadtree.
	playerQuadtree.Add_WLq(up, up.GetPreviousPos()) // Add self
	up.connState = PlayerConnStateIn
	up.updatedStats = true // Make sure the client gets to know the player stats.
	// fmt.Println("LoginCommand OCTREE: ", playerQuadtree)

	var friends bytes.Buffer
	allPlayersSem.Lock()
	allPlayerNameMap[strings.ToLower(up.pl.name)] = up
	allPlayerIdMap[up.uid] = up
	// Look for friends and tell them
	for _, uid := range up.pl.Listeners {
		other, ok := allPlayerIdMap[uid]
		if ok {
			other.Printf("Logged in: %v", up.pl.name)
			if friends.Len() > 0 {
				friends.WriteString(", ")
			}
			friends.WriteString(other.pl.name)
		}
	}
	allPlayersSem.Unlock()
	if friends.Len() > 0 {
		up.Printf_Bl("Friends %s", friends.String())
	}
}

func CmdClose_BlWLqWLuWLa(i int) {
	up := allPlayers[i]
	if up == nil {
		log.Panicf("CmdClose index %s (%d) already closed\n", up.pl.name, i)
	}
	if *verboseFlag > 1 {
		log.Println("CmdClose", up.pl.name)
	}
	// up.Printf_Bl("Goodbye %s!", up.pl.name) // This print may fail if connection is already closed.
	if up.connState == PlayerConnStateIn || up.connState == PlayerConnStateDisc {
		// There is a small chance that the player was just being moved. In that case, the player object
		// remains at the previous position, which is where it will be found in the quadtree.
		playerQuadtree.Remove_WLq(up)
	}
	// TODO: Should tell near players of this?
	up.Lock()
	up.conn.Close()
	up.connState = PlayerConnStateLogin // Default, even though this one is going to be disconnected.
	up.Unlock()

	allPlayersSem.Lock()
	numPlayers--
	delete(allPlayerNameMap, strings.ToLower(up.pl.name)) // Clear association from player name to index
	delete(allPlayerIdMap, up.uid)                        // Cleanh assocition from player uid to index
	for _, uid := range up.pl.Listeners {
		other, ok := allPlayerIdMap[uid]
		if ok {
			other.Printf("Logged out: %v", up.pl.name)
		}
	}
	allPlayers[i] = nil
	allPlayersSem.Unlock()
}

func CmdSavePlayerNow_RluBl(index int) {
	up := allPlayers[int(index)]
	up.RLock()
	if up.lic != nil && up.connState == PlayerConnStateIn {
		up.pl.Save_Bl()
	}
	up.RUnlock()
	return
}

// Send a text message to a player, which must not be locked.
// Wait until sure the message has been sent.
func (up *user) Printf_Bl(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	length := len(str) + 3
	prefix := []byte{byte(length & 0xFF), byte(length >> 8), client_prot.CMD_MESSAGE}
	msg := string(prefix) + str
	up.writeBlocking_Bl([]byte(msg))
}

// Report the coordinate to the current client.
func (up *user) CmdReportCoordinate_RLuBl(lockedElsewhere bool) {
	const length = 3*8 + 3
	var ans [length]byte
	ans[0] = byte(length & 0xFF)
	ans[1] = byte(length >> 8)
	ans[2] = client_prot.CMD_REPORT_COORDINATE
	// fmt.Printf("CmdReportCoordinate: Reporting coordinate (%v)\n", up.pl.coord)
	if !lockedElsewhere {
		up.RLock() // TODO: This is not pretty.
	}
	EncodeUint64(uint64(int64(up.pl.coord.X*client_prot.BLOCK_COORD_RES)), ans[3:11])
	EncodeUint64(uint64(int64(up.pl.coord.Y*client_prot.BLOCK_COORD_RES)), ans[11:19])
	EncodeUint64(uint64(int64(up.pl.coord.Z*client_prot.BLOCK_COORD_RES)), ans[19:27])
	if !lockedElsewhere {
		up.RUnlock()
	}
	up.writeBlocking_Bl(ans[:])
}

func CmdAttachBlock_WLwWLcRLq(cc chunkdb.CC, dx, dy, dz uint8, blType block, index int) {
	cp := ChunkFind_WLwWLc(cc)
	from := allPlayers[index]
	if cp.owner != from.uid && from.pl.adminLevel < 1 {
		from.Printf("Not owner of chunk. See help for territory")
		return
	}
	if !cp.UpdateBlock_WLcWLw(dx, dy, dz, blType) {
		return
	}
	from.pl.blockAdd += 1
	// fmt.Println("CmdAttachBlock: ", abc.index, "Chunk: ", abc.cc, "Offset: ", abc.dx, abc.dy, abc.dz, "type: ", abc.blType)
	// Send command of updated block to the player.
	near := playerQuadtree.FindNearObjects_RLq(from.GetPreviousPos(), client_prot.NEAR_OBJECTS)
	// fmt.Printf("Near objects to %v: %v\n", up.pl.coord, near)
	// fmt.Println("CmdAttachBlock OCTREE: ", playerQuadtree)
	// Tell anyone near that a block has changed, including self.
	for _, o := range near {
		// Only need to tell players, not monsters etc.
		up, ok := o.(*user)
		if ok {
			up.SendMessageBlockUpdate(cc, dx, dy, dz, blType)
		}
	}
}

// Compose a message to the client to update player stats. The player must not be locked.
// The argument is a buffer that will be used for the message. The only reason for this is to
// avoid allocating new buffers every time.
func (up *user) SendMsgUpdatedStats_Bl(ans []byte) {
	const length = 14
	ans[0] = length
	ans[1] = 0 // MSB of length
	ans[2] = client_prot.CMD_PLAYER_STATS
	ans[3] = byte(up.pl.hitPoints * 255)
	ans[4] = byte(up.pl.exp * 255)
	EncodeUint32(up.pl.level, ans[5:9])
	EncodeUint32(up.flags, ans[9:13])
	ans[13] = byte(up.pl.mana * 255)
	up.writeBlocking_Bl(ans[:length]) // This message must not be lost.
}

// Remove a block from a chunk. That is, replace it with air.
func (up *user) HitBlock_WLwWLcRLq(cc chunkdb.CC, dx, dy, dz uint8) {
	// TODO: Check distance to player, only allow digging near blocks.
	cp := ChunkFind_WLwWLc(cc)
	if cp.owner != up.uid && up.pl.adminLevel < 1 {
		up.Printf_Bl("#FAIL Not owner of chunk. See help for territory")
		return
	}

	// Is this actually a teleport that shall be removed? It is a special case, as teleports are not
	// stored as blocks in the chunk.
	tx, ty, tz, teleport := superChunkManager.GetTeleport(&cc)
	if teleport && dx == tx && dy == ty && dz == tz {
		superChunkManager.RemoveTeleport(&cc)
		f := func(up *user) {
			up.SuperChunkAnswer_Bl(&cc)
			up.SendMessageBlockUpdate(cc, tx, ty, tz, BT_Air)
		}
		up.pl.coord.CallNearPlayers_RLq(f, nil)
		return
	}

	if !cp.UpdateBlock_WLcWLw(dx, dy, dz, BT_Air) {
		return
	}
	up.pl.blockRem += 1
	// fmt.Println("CmdHitBlock: ", hbc.index, "Chunk: ", hbc.cc, "Offset: ", hbc.dx, hbc.dy, hbc.dz)
	// fmt.Println(ans)
	// Find near players and tell them about the change.
	near := playerQuadtree.FindNearObjects_RLq(up.GetPreviousPos(), client_prot.NEAR_OBJECTS)
	// fmt.Printf("CmdHitBlock: Near objects to %v: %v\n", up.pl.coord, near)
	// Tell anyone near that a block has changed, including self.
	for _, o := range near {
		other, ok := o.(*user)
		// No need to tell anything else but players.
		if ok {
			other.SendMessageBlockUpdate(cc, dx, dy, dz, BT_Air)
		}
	}
}

// The client asked to verify the checksum of chunk.
// The length of the remaining message has already been checked before the call.
func CommandVerifyChunkCS_WLwWLcBl(i int, b []byte) {
	for len(b) >= 7 {
		xLSB := b[0]
		yLSB := b[1]
		zLSB := b[2]
		up := allPlayers[i]
		coord := up.pl.coord.GetChunkCoord().UpdateLSB(xLSB, yLSB, zLSB)
		ch := ChunkFind_WLwWLc(coord)
		// Error here does not cause any problems, other than that the chunk is sent
		vfysum, _, _ := ParseUint32(b[3:7])

		if ch.checkSum != vfysum {
			//fmt.Printf("CommandVerifyChunkCS mismatch: %v player coord %v, checksum %v\n", i, up.pl.coord, ch.checkSum)
			up.CmdReadChunk_WLwWLcBl(coord) // Use exisiting method to send chunk
		} else {
			// If the checksum was correct, we will do nothing!
			//fmt.Printf("Checksum request match!\n")
		}

		b = b[7:]
	}
}

func (up *user) SuperChunkAnswer_Bl(cc *chunkdb.CC) {
	const length = 4010
	var msg [3]byte
	msg[0] = length & 0xFF
	msg[1] = length >> 8
	msg[2] = client_prot.CMD_SUPERCHUNK_ANSWER
	up.writeBlocking_Bl(msg[:])
	superChunkManager.Write(up.conn, cc)
}

// The client asked to verify the checksum of chunk.
// The length of the remaining message has already been checked before the call.
func (up *user) CommandVerifySuperchunkCS_Bl(b []byte) {
	for len(b) >= 7 {
		xLSB := b[0]
		yLSB := b[1]
		zLSB := b[2]
		coord := up.pl.coord.GetChunkCoord().UpdateLSB(xLSB, yLSB, zLSB)

		// Error here does not cause any problems, other than that the chunk is sent
		vfysum, _, _ := ParseUint32(b[3:7])

		if !superChunkManager.VerifyChecksum(&coord, vfysum) {
			up.SuperChunkAnswer_Bl(&coord)
		}

		b = b[7:]
	}
}

func (up *user) CmdReadChunk_WLwWLcBl(cc chunkdb.CC) {
	// Check that it is a valid request, inside a limited range. We don't want clients to
	// spam the server with requests, and we don't want them to download the complete world.
	userCC := up.prevCoord.GetChunkCoord()
	dx := userCC.X - cc.X
	dy := userCC.Y - cc.Y
	dz := userCC.Z - cc.Z
	dist := dx*dx + dy*dy + dz*dz
	if dist > 3*CnfgMaxChunkReqDist*CnfgMaxChunkReqDist {
		// Factor 3 is need for worst case with maximum distance in all three dimensions.
		log.Println("User", up.pl.name, "requested chunk too far away", userCC, cc, "distance", dist)
		up.Printf("!Bad chunk request")
		return
	}

	var ch []byte
	var ans [15 + 3*4]byte
	{
		// 'b' is in a local block to make sure 'b' isn't accessed outside of read lock.
		b := ChunkFind_WLwWLc(cc)
		b.RLock()
		// The compressed data is ok to save for access outside of lock, as it will not be updated by anyone else.
		// It may be that a new compressed block is allocated, in which case the old one will be saved here.
		ch = b.ch_comp
		EncodeUint32(b.flag, ans[3:7])
		EncodeUint32(b.checkSum, ans[7:11])
		EncodeUint32(b.owner, ans[11:15])
		b.RUnlock() // Clear the lock before writing, which may possibly block for a while.
	}
	length := cap(ans) + len(ch)
	ans[0] = byte(length & 0xFF)
	ans[1] = byte(length >> 8)
	ans[2] = client_prot.CMD_CHUNK_ANSWER
	EncodeUint32(uint32(cc.X), ans[15:19])
	EncodeUint32(uint32(cc.Y), ans[19:23])
	EncodeUint32(uint32(cc.Z), ans[23:27])
	// Send the header of the message
	up.writeBlocking_Bl(ans[:])
	// Send the payload of the message
	up.writeBlocking_Bl(ch)
}

func (uc *user_coord) NearLadder_WLwWLc() bool {
	west := DBGetBlockCached_WLwWLc(user_coord{uc.X - 1, uc.Y, uc.Z + 1})
	east := DBGetBlockCached_WLwWLc(user_coord{uc.X + 1, uc.Y, uc.Z + 1})
	south := DBGetBlockCached_WLwWLc(user_coord{uc.X, uc.Y - 1, uc.Z + 1})
	north := DBGetBlockCached_WLwWLc(user_coord{uc.X, uc.Y + 1, uc.Z + 1})
	if west == BT_Ladder || east == BT_Ladder || south == BT_Ladder || north == BT_Ladder {
		return true
	}
	return false
}

// To simplify, only one command type is used for moving, with an argument that describes
// what movement type it is. To make it easy, simply use the client_prot.xxx
// Locks aren't needed in general, as it is always ok to change these flags anytime.
// When a start moving forward key is activated, the opposite direction is stopped
// at the same time. Thsi should not be needed, as a stop signal will be generated
// by the client when the player releases the key. But, sometimes this key-up event
// is missed (focus leaving window, etc).
func (up *user) CmdPlayerMove_WLuWLqWLmWLwWLc(cmd int) {
	// log.Printf("Player %d move command %d\n", i, cmd)
	checktrigger := false
	bl, swimming := up.cmdUpdatePosition_WLuWLqWLmWLwWLc() // This will update player position and reset the current timers
	switch cmd {
	case client_prot.CMD_JUMP:
		up.Lock()
		if !up.pl.flying && up.pl.coord.NearLadder_WLwWLc() {
			up.pl.climbing = true
		} else {
			up.pl.climbing = false
		}
		if up.pl.climbing {
			feet := up.pl.coord
			feet.Z += PlayerHeight + 1
			if blockIsPermeable[DBGetBlockCached_WLwWLc(feet)] {
				up.pl.coord.Z += 1
				checktrigger = true
				bl = DBGetBlockCached_WLwWLc(up.pl.coord)
			}
		} else if up.pl.flying || swimming {
			head1 := up.pl.coord
			head1.Z += PlayerHeight + 1
			head2 := head1
			head2.Z++
			if blockIsPermeable[DBGetBlockCached_WLwWLc(head1)] && blockIsPermeable[DBGetBlockCached_WLwWLc(head2)] {
				up.pl.coord.Z += 2
			}
			checktrigger = true
		} else if !blockIsPermeable[DBGetBlockCached_WLwWLc(user_coord{up.pl.coord.X, up.pl.coord.Y, up.pl.coord.Z - 0.1})] {
			up.pl.ZSpeed = PlayerJumpSpeed
		}
		up.Unlock()
	case client_prot.CMD_START_FWD:
		up.mvFwd = true
		up.mvBwd = false
	case client_prot.CMD_STOP_FWD:
		up.mvFwd = false
	case client_prot.CMD_START_BWD:
		up.mvBwd = true
		up.mvFwd = false
	case client_prot.CMD_STOP_BWD:
		up.mvBwd = false
	case client_prot.CMD_START_LFT:
		up.mvLft = true
		up.mvRgt = false
	case client_prot.CMD_STOP_LFT:
		up.mvLft = false
	case client_prot.CMD_START_RGT:
		up.mvRgt = true
		up.mvLft = false
	case client_prot.CMD_STOP_RGT:
		up.mvRgt = false
	}

	if checktrigger && up.pl.flying && up.pl.adminLevel == 0 {
		// Only check flying if player moved, to save effort
		cp := ChunkFindCached_WLwWLc(up.pl.coord.GetChunkCoord())
		if cp == nil || cp.owner != up.uid {
			up.pl.flying = false
		}
	}

	if checktrigger {
		up.CheckAndActivateTriggers_WLwWLuWLqWLmWLc(bl)
	}
}

// This function is called regularly, but also at events generated by the player hitting or releasing
// the movement key.
// Return:
//    bl           The block at the player feet
//    swimming     True if player is swimming
func (up *user) cmdUpdatePosition_WLuWLqWLmWLwWLc() (bl block, swimming bool) {
	var checktrigger bool
	up.Lock()
	checktrigger, bl, swimming = up.cmdUpdatePosition2_WLwWLc()
	up.Unlock()

	if checktrigger {
		up.CheckAndActivateTriggers_WLwWLuWLqWLmWLc(bl)
	}

	return
}

// Define how much of the body will be above water when swimming.
const SWIMMINGHEIGHT = PlayerHeight / 2

// Find out if main body is in water, in which case we consider the player swimming
func (coord *user_coord) swimming() bool {
	// Find out if main body is in water, in which case we consider the player swimming
	bodyPos := user_coord{coord.X, coord.Y, coord.Z + SWIMMINGHEIGHT}
	bodyBlock := DBGetBlockCached_WLwWLc(bodyPos)
	return bodyBlock == BT_Water || bodyBlock == BT_BrownWater
}

var delayMovementScoreUpdate uint16

// Return as follows:
// * true if the player moved
// * the new block type at the feet (if moved)
// * true if swimming
func (up *user) cmdUpdatePosition2_WLwWLc() (bool, block, bool) {
	// TODO: This function is called very frequently (10 times/s), for every player. If the player isn't moving, and the environment
	// doesn't change (trap door opening), there is no need to update the position.
	if up.connState != PlayerConnStateIn { // Atomical read, no lock needed
		return false, 0, false
	}
	// Compute the player relative movement first, then transpose it to world coordinates
	var x, y, z float64

	// There is a chance that more than one condition will update deltaTime, but they
	// will give the same result as this function is called everytime a change in movement happens.
	now := time.Now()
	deltaTime := now.Sub(up.startMoving) // The time the player has been moving.
	up.startMoving = now

	if up.pl.flying || (up.pl.climbing && !up.pl.coord.NearLadder_WLwWLc()) {
		up.pl.climbing = false
	}

	swimming := up.pl.coord.swimming()

	noGravity := swimming || up.pl.flying || up.pl.climbing
	var sv, cv float64 = 0, 1 // Sin and Cos for the vertical angle
	if !noGravity {
		// Apply gravity
		newSpeed := UpdateZPos_WLwWLc(deltaTime, up.pl.ZSpeed, &up.pl.coord)
		if up.pl.ZSpeed < -1.0 && newSpeed == 0 {
			// Tell client that the player hit the ground with some speed (corresponding to falling two blocks)
			up.flags |= client_prot.UserFlagJump
			up.updatedStats = true
		}
		up.pl.ZSpeed = newSpeed
		// fmt.Println("New falling speed ", up.pl.ZSpeed, " at pos ", up.pl.coord.Z)
		z = 0
	} else {
		sv, cv = math.Sincos(float64(up.pl.dirVert))
		if up.mvFwd {
			z = -sv
		} else if up.mvBwd {
			z = sv
		}
	}

	if up.mvFwd {
		y = cv
	}
	if up.mvBwd {
		y = -cv
	}
	if up.mvLft {
		x = -cv
	}
	if up.mvRgt {
		x = cv
	}

	if x == 0 && y == 0 && z == 0 {
		// Player isn't moving
		return false, 0, swimming
	}

	// Normalize the xyz vector.
	d := math.Sqrt(x*x + y*y + z*z)
	dist := float64(deltaTime) / 1e9 * RUNNING_SPEED // Now scale distance
	if up.pl.flying {
		dist *= FlyingSpeedFactor // Flying is quicker than running
	}
	// fmt.Printf("cmdUpdatePosition player %d moving %.2f,%.2f,%.2f. d=%.2f, dist=%.2f, delta=%.3f, zspeed %v\n", up.id, x, y, z, d, dist, float64(deltaTime)/1e9, up.pl.ZSpeed)
	// Apply scaling in case of moving diagonally.
	x *= dist / d
	y *= dist / d
	z *= dist / d
	// Rotate, according to looking direction. North is 0 radians, increasing angle for turning to the right
	s, c := math.Sincos(float64(-up.pl.dirHor))
	x2 := x*c - y*s
	y2 := x*s + y*c
	z2 := z
	// log.Printf("Player moved from %d,%d to ", up.pl.coord.X, up.pl.coord.Y)
	var newCoord user_coord = user_coord{up.pl.coord.X + x2, up.pl.coord.Y + y2, up.pl.coord.Z + z2}
	bl := DBGetBlockCached_WLwWLc(newCoord)
	if blockIsPermeable[bl] || (up.pl.adminLevel == 10 && up.pl.flying) {
		// There was room for the feet, at least. Now check up to head height
		// A flying admin will always succeed, which will allow him to fly through ground.
		delta := newCoord
		for off := 1.0; off < PlayerHeight; off += 1.0 {
			delta.Z += 1.0
			if !blockIsPermeable[DBGetBlockCached_WLwWLc(delta)] && !(up.pl.adminLevel == 10 && up.pl.flying) {
				// Sorry, hitting a roof here.
				return false, 0, swimming
			}
		}

		if swimming && !newCoord.swimming() {
			// Check if the player was swimming upwards, and is now just above the water level
			belowFeet := newCoord
			belowFeet.Z--
			bl := DBGetBlockCached_WLwWLc(belowFeet)
			if bl == BT_Water || bl == BT_BrownWater {
				// Player is still swimming, but got a little too high. Round it downwards, with a little
				// delta to make sure the player stays in the water.
				newCoord.Z = math.Floor(newCoord.Z+SWIMMINGHEIGHT) - SWIMMINGHEIGHT - 0.1
			}
		}
		// The move has now been approved.
		up.pl.coord = newCoord

		// Update the score of this place, but not every time (to save some performance)
		const DelayMovementReportFactor = 10
		if delayMovementScoreUpdate++; delayMovementScoreUpdate == DelayMovementReportFactor {
			delayMovementScoreUpdate = 0
			cp := ChunkFindCached_WLwWLc(newCoord.GetChunkCoord())
			owner := cp.owner
			if owner != up.uid && !up.pl.dead && owner != OWNER_NONE && owner != OWNER_RESERVED && owner != OWNER_TEST && up.uid < math.MaxUint32/2 {
				up.AddScore(owner, CnfgScoreMoveFact*DelayMovementReportFactor*dist)
			}
		}
		return true, bl, swimming
	}

	// There is some obstacle. Allow the movement if it is one block of height difference.
	oneStepUp := user_coord{newCoord.X, newCoord.Y, newCoord.Z + 1}
	bl = DBGetBlockCached_WLwWLc(oneStepUp)
	if blockIsPermeable[bl] {
		// There was room for the feet, at least. Now check up to head height
		delta := oneStepUp
		for off := 1.0; off < PlayerHeight; off += 1.0 {
			delta.Z += 1.0
			if !blockIsPermeable[DBGetBlockCached_WLwWLc(delta)] {
				// Sorry, hitting a roof here.
				return false, 0, swimming
			}
		}
		up.pl.coord = oneStepUp
		return true, bl, swimming
	}
	return false, 0, swimming
}

// Restore mana. Return true if any restore was needed.
func (up *user) Mana(mana float32) (ret bool) {
	if up.pl.mana+mana > 1 {
		mana = 1 - up.pl.mana
	}
	if mana > 0 {
		up.pl.mana += mana
		// up.flags |= client_prot.UserFlagHealed
		up.updatedStats = true
		ret = true
	}
	return
}

// Heal the player. Return true if any healing was needed
func (up *user) Heal(heal, manaCost float32) (ret bool) {
	if up.pl.hitPoints+heal > 1 {
		heal = 1 - up.pl.hitPoints
	}
	if heal > 0 {
		up.pl.hitPoints += heal
		up.pl.mana -= manaCost
		up.flags |= client_prot.UserFlagHealed
		up.updatedStats = true
		ret = true
	}
	return
}

// Manage player spells
func (up *user) CmdPlayerAction_WLuBl(userAction byte) {
	if up.pl.dead {
		up.Printf_Bl("Unable now\n")
		return
	}
	switch userAction {
	case client_prot.UserActionHeal:
		if up.pl.mana < CnfgManaForHealing {
			up.Printf_Bl("Not enough mana")
		} else if up.pl.hitPoints == 1 {
			up.Printf_Bl("Already full health")
		} else {
			heal := float32(CnfgHealthAtHealingSpell)
			up.Lock()
			up.Heal(heal, CnfgManaForHealing)
			up.Unlock()
		}
	case client_prot.UserActionCombAttack:
		mp := up.aggro
		if mp == nil {
			up.Printf_Bl("Start attack first")
		} else if up.pl.mana < CnfgManaForCombAttack {
			up.Printf_Bl("Not enough mana")
		} else if !mp.dead {
			up.Lock()
			up.pl.mana -= CnfgManaForCombAttack
			up.updatedStats = true
			up.Unlock()
			mp.Hit_WLuBl(up, CnfgWeaponDmgCombAttack)
		}
	}
}

// Inititate attack on a monster
func (up *user) CmdAttackMonster_WLuRLm(b []byte) {
	monsterId, b, ok := ParseUint32(b)
	if !ok {
		log.Printf("Failed to parse monster level %v\n", b)
		return
	}
	monsterData.RLock()
	mp := monsterData.m[monsterId]
	monsterData.RUnlock()
	if mp == nil {
		// Monster doesn't exist any longer
		return
	}
	if up.pl.dead {
		up.Printf_Bl("Can't attack when dead\n")
		return
	}
	dx := up.pl.coord.X - mp.coord.X
	dy := up.pl.coord.Y - mp.coord.Y
	dz := up.pl.coord.Z - mp.coord.Z
	if dx*dx+dy*dy+dz*dz > CnfgMonsterAggroDistance*CnfgMonsterAggroDistance {
		// Too far away.
		up.Printf_Bl("Too far away to attack.")
		return
	}
	if mp.aggro == nil {
		// Done without a lock, which means we could overwrite another aggro. This is non fatal.
		mp.aggro = up
		mp.speed = ASSAULT_FACTOR * mp.maxSpeed // Increase speed of monster while attacking
		mp.state = MD_DEFENDING
	}
	if up.aggro == mp {
		// Attack has already been initiated with this specific monster.
		return
	}
	up.Lock()
	up.aggro = mp
	up.flags |= client_prot.UserFlagInFight
	up.updatedStats = true // Tell the client that we are now in fight mode
	up.Unlock()
	up.Printf_Bl("You attack a level %d monster, %.0f%% hp", mp.Level, mp.HitPoints*100)
}

func CmdSetDirections(index int, dirHor, dirVert float32) {
	// Locking isn't used here. What can happen is that the player gets disconnected,
	// but then noone cares about directions. The horisontal and vertical looking direction
	// do not depend on each other, so there is no danger if one is udpated and the other
	// is not.
	up := allPlayers[index]
	up.pl.dirHor, up.pl.dirVert = dirHor, dirVert
	// log.Printf("SetDirectionsCommand: Player %v new looking dir: %v,%v\n", sdc.index, up.dirHor, up.dirVert)
}

func (up *user) MessageMoved(coord *user_coord) {
	// Find near players and tell them 'up' has moved
	near := playerQuadtree.FindNearObjects_RLq(&TwoF{coord.X, coord.Y}, client_prot.NEAR_OBJECTS)
	// fmt.Printf("Near objects to %v: %v\n", up.pl.coord, near)
	// fmt.Println("ClientUpdatePlayerPosCommand OCTREE: ", playerQuadtree)
	for _, o := range near {
		other, ok := o.(*user)
		if other == up || !ok {
			// Found self, or non-player. Don't tell monsters, they
			// don't care if players move or not.
			continue
		}
		other.Lock()
		other.SomeoneMoved(up)
		other.Unlock()
	}
}

// If a player has moved, tell everyone around.
func (up *user) checkOnePlayerPosChanged_RLuWLqBl(forceUpdate bool) {
	// No read lock used, giving a small chance that the coordinate changed.

	if up.connState != PlayerConnStateIn {
		return
	}

	dest := up.pl.coord
	moved := up.prevCoord.X != dest.X || up.prevCoord.Y != dest.Y || up.prevCoord.Z != dest.Z
	if moved {
		// Tell self
		up.CmdReportCoordinate_RLuBl(false)
	}
	// TODO: Use a flag instead to mark that the player moved.
	if moved || forceUpdate {
		// fmt.Printf("Player %d moved to %v\n", index, up.pl.coord)
		playerQuadtree.Move_WLq(up, &TwoF{dest.X, dest.Y}) // This will also update the previous coordinate
		up.prevCoord = dest
		up.MessageMoved(&up.prevCoord)
		// Find near players and tell them 'up' has moved
	}
}

// For user 'up', send a message to the client of what near objects have moved.
func clientTellMovedObjects_Bl(up *user) {
	// fmt.Printf("clientTellMovedObjects: %+v\n", up)
	const MaxListLength = 200 // As specified in the protocol
	listMoved := up.objMoved
	up.objMoved = up.objMoved[0:0] // Empty the list
	// The content of the list can change as there is no lock. This is acceptable,
	// as writing to the client can block. In worst case, some movements are lost.
	if up.connState != PlayerConnStateIn || len(listMoved) == 0 {
		// The requirement is that 'up' must be connected to a client, the
		// player must be logged in, and there must be a list of objects that moved.
		return
	}
	b := make([]byte, MaxListLength)
	length := 3 // Initial value, counting the header of the message
	b[2] = client_prot.CMD_OBJECT_LIST
	const lengthPerObject = 18
	for _, o := range listMoved {
		EncodeUint32(o.GetId(), b[length:length+4])
		b[length+4] = client_prot.ObjStateInGame
		b[length+5] = o.GetType()
		var objLevel uint32 = 0
		var objHP uint8 = 0
		switch o2 := o.(type) {
		case *user:
			objLevel = o2.pl.level
			objHP = uint8(o2.pl.hitPoints * 255)
		case *monster:
			objLevel = o2.Level
			objHP = uint8(o2.HitPoints * 255)
			// fmt.Printf("clientTellMovedObjects: %#v\n", o)
		}
		b[length+6] = objHP
		EncodeUint32(objLevel, b[length+7:length+11])
		pos := o.GetPreviousPos()
		// Encode the relative coordinates, scaled by BLOCK_COORD_RES
		EncodeUint16(uint16(int16((pos[0]-up.pl.coord.X)*client_prot.BLOCK_COORD_RES)), b[length+11:length+13])
		EncodeUint16(uint16(int16((pos[1]-up.pl.coord.Y)*client_prot.BLOCK_COORD_RES)), b[length+13:length+15])
		EncodeUint16(uint16(int16((o.GetZ()-up.pl.coord.Z)*client_prot.BLOCK_COORD_RES)), b[length+15:length+17])
		b[length+17] = byte(256 / 2 / math.Pi * o.GetDir()) // Convert direction into range 0-255
		length += lengthPerObject
		if length+lengthPerObject > cap(b) {
			// Can't fit another object in the list, send what there is
			EncodeUint16(uint16(length), b[0:2])
			up.writeBlocking_Bl(b[0:length]) // This will use 'b', so we need a allocate a new buffer
			b = make([]byte, MaxListLength)
			b[2] = client_prot.CMD_OBJECT_LIST
			length = 3 //  Start a new message
		}
	}
	if length > 3 {
		// If any remaining, send it. This is the usual case
		EncodeUint16(uint16(length), b[0:2])
		// fmt.Printf("ClientUpdatePlayerPosCommand moved objects for player %d: %v\n", i, b[0:length])
		up.writeBlocking_Bl(b[0:length])
	}
}

func (up *user) GetZ() float64 {
	return up.pl.coord.Z
}

func (*user) GetType() uint8 {
	return client_prot.ObjTypePlayer
}

func (up *user) GetId() uint32 {
	return up.uid
}

// Get looking direction in radians
func (this *user) GetDir() float32 {
	return this.pl.dirHor
}

// Do not call conn.Write() directly elsewhere.
// This function can block if the receiver is not quick enough to read. Because of that, only the client process,
// which can be allowed to block, should do the call. Another reason is that the writing is not atomic.
var WorstWriteTime time.Duration

func (up *user) writeBlocking_Bl(b []byte) {
	if up.connState == PlayerConnStateDisc {
		// Connection no longer available, don't even try
		return
	}
	for len(b) > 0 {
		now := time.Now()
		n, err := up.conn.Write(b)
		trafficStatistics.AddSend(len(b))
		diff := time.Now().Sub(now)
		if diff > WorstWriteTime {
			WorstWriteTime = diff
		}
		if err == nil {
			if *verboseFlag > 2 {
				if n == len(b) {
					log.Printf("Blocking Send to %v '%v'\n", up.pl.name, b[0:3])
				} else {
					log.Printf("partial send to %v '%v'\n", up.pl.name, b[0:3])
				}
			}
			b = b[n:]
		} else if e2, ok := err.(*net.OpError); ok && (e2.Temporary() || e2.Timeout()) {
			continue
		} else {
			// There could be a failure because of multiple parallel actions (disconnecting while also sending new messages)
			if *verboseFlag > 1 && up.connState == PlayerConnStateIn {
				log.Printf("writeBlocking_Bl %v %#v\n", err, err)
			}
			if up.connState == PlayerConnStateIn {
				// Only change to connected state if the player also was logged in.
				up.connState = PlayerConnStateDisc
			}
			return
		}
	}
	return
}

// The player data must be write locked
func (up *user) AddExperience(e float32) {
	up.pl.exp += e
	if up.pl.exp > 1 {
		up.pl.level += 1
		up.pl.exp -= 1
	}
	up.updatedStats = true
}

// Add a 'up' to listerens of player 'name'. 'name' must be logged in.
func (up *user) AddToListener_RLaWLu(name string) (notFound bool, alreadyIn bool) {
	allPlayersSem.RLock()
	other, ok := allPlayerNameMap[strings.ToLower(name)]
	allPlayersSem.RUnlock()
	if ok {
		other.Lock()
		if other.connState == PlayerConnStateIn {
			// Check first if 'up' already is in the list
			for i := 0; i < len(other.pl.Listeners); i++ {
				if other.pl.Listeners[i] == up.uid {
					alreadyIn = true
					break
				}
			}
			if !alreadyIn {
				other.pl.Listeners = append(other.pl.Listeners, up.uid)
			}
		} else {
			ok = false
		}
		other.Unlock()
	}
	notFound = !ok
	return
}

// Add a 'up' to listerens of player 'name'. 'name' must be logged in.
func (up *user) RemoveFromListener_RLaWLu(name string) (notFound bool, notIn bool) {
	allPlayersSem.RLock()
	other, ok := allPlayerNameMap[strings.ToLower(name)]
	allPlayersSem.RUnlock()
	notIn = true
	if ok {
		other.Lock()
		if other.connState == PlayerConnStateIn {
			// Check first if 'up' already is in the list
			var i int
			for i = 0; i < len(other.pl.Listeners); i++ {
				if other.pl.Listeners[i] == up.uid {
					notIn = false
					break
				}
			}
			if !notIn {
				l := other.pl.Listeners
				if i < len(l)-1 {
					// Remove item 'i' by replacing it with the last item
					l[i] = l[len(l)-1]
				}
				other.pl.Listeners = l[:len(l)-1]
			}
		} else {
			ok = false
		}
		other.Unlock()
	}
	notFound = !ok
	return
}

func (up *user) FileMessage(fileName string) {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		// Non fatal problem, silently give it up
		return
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if len(line) != 0 {
			// Ignore empty lines
			up.Printf_Bl(line)
		}
	}
}

// Report the complete inventory, to the current player. This is not the same as the equipped items.
func (up *user) ReportAllInventory_WluBl() {
	up.RLock()
	inv := up.pl.inventory
	l := inv.Len()
	const N = 9
	msgLen := l*N + 3
	var b = make([]byte, msgLen)
	b[0] = byte(msgLen & 0xff)
	b[1] = byte(msgLen >> 8)
	b[2] = client_prot.CMD_UPD_INV
	for i := 0; i < l; i++ {
		obj := inv.Get(i)
		str := []byte(obj.ID())
		if *verboseFlag > 1 {
			log.Printf("%s %#v %s\n", obj.ID(), obj, str)
		}
		for j := 0; j < 4; j++ {
			b[i*N+3+j] = str[j]
		}
		count := obj.GetCount()
		if count > math.MaxUint8 {
			count = math.MaxUint8 // This is what can be shown to the client
		}
		b[i*N+7] = count
		EncodeUint32(obj.GetLevel(), b[i*N+8:i*N+12])
	}
	up.RUnlock()
	if l > 0 {
		// Don't bother sending a message if there was no inventory
		if *verboseFlag > 2 {
			log.Println(b)
		}
		up.writeBlocking_Bl(b)
	}
}

// Create any items that was dropped by a monster.
// 'modifier' is a probability modifier. 1 is normal probabilities, less than 1 is higher.
// Using 'combatExperienceSameLevel' as a base reference should produce one drop on average every level.
func (up *user) MonsterDropWLu(modifier float32) {
	// log.Println("Modifier", modifier, "exp same level", combatExperienceSameLevel)
	if rand.Float32()*modifier < 0.05 {
		AddOneObjectToUser_WLuBl(up, MakeHealthPotion(0))
	}
	if rand.Float32()*modifier < 0.05 {
		AddOneObjectToUser_WLuBl(up, MakeManaPotion(0))
	}

	// Make weapons
	prob := rand.Float32() * modifier
	if prob < combatExperienceSameLevel/100 {
		AddOneObjectToUser_WLuBl(up, MakeWeapon3(up.pl.level))
	} else if prob < combatExperienceSameLevel/10 {
		AddOneObjectToUser_WLuBl(up, MakeWeapon2(up.pl.level))
	} else if prob < combatExperienceSameLevel {
		AddOneObjectToUser_WLuBl(up, MakeWeapon1(up.pl.level))
	}

	// Make armors
	prob = rand.Float32() * modifier
	if prob < combatExperienceSameLevel/100 {
		AddOneObjectToUser_WLuBl(up, MakeArmor3(up.pl.level))
	} else if prob < combatExperienceSameLevel/10 {
		AddOneObjectToUser_WLuBl(up, MakeArmor2(up.pl.level))
	} else if prob < combatExperienceSameLevel {
		AddOneObjectToUser_WLuBl(up, MakeArmor1(up.pl.level))
	}

	// Make helmets
	prob = rand.Float32() * modifier
	if prob < combatExperienceSameLevel/100 {
		AddOneObjectToUser_WLuBl(up, MakeHelmet3(up.pl.level))
	} else if prob < combatExperienceSameLevel/10 {
		AddOneObjectToUser_WLuBl(up, MakeHelmet2(up.pl.level))
	} else if prob < combatExperienceSameLevel {
		AddOneObjectToUser_WLuBl(up, MakeHelmet1(up.pl.level))
	}
}

// Report current equipment of 'up' to 'target'.
func (target *user) ReportEquipment_Bl(up *user) {
	// No lock is used. That means that the equipment can change over time, but this is not fatal.
	const msgLen = 34
	var b [msgLen]byte
	b[0] = msgLen
	b[1] = 0
	b[2] = client_prot.CMD_EQUIPMENT
	EncodeUint32(up.uid, b[3:7])
	b[7] = 0 // Slot, 0 means weapon
	copy(b[8:12], ConvertWeaponTypeToID(up.pl.WeaponType))
	EncodeUint32(up.pl.WeaponLvl, b[12:16])
	b[16] = 1 // Slot, 1 means armor
	copy(b[17:21], ConvertArmorTypeToID(up.pl.ArmorType))
	EncodeUint32(up.pl.ArmorLvl, b[21:25])
	b[25] = 2 // Slot, 2 means helmet
	copy(b[26:30], ConvertHelmetTypeToID(up.pl.HelmetType))
	EncodeUint32(up.pl.HelmetLvl, b[30:34])
	if target == up {
		target.writeBlocking_Bl(b[:])
	} else {
		target.writeNonBlocking(b[:])
	}
	// log.Println("From", up.pl.name, "to", target.pl.name, b)
}

// Fulfill the Writer interface. This is only used for writing strings.
func (up *user) Write(p []byte) (n int, err error) {
	length := len(p)
	if p[length-1] == '\n' {
		// Strip trailing newlines
		length--
	}
	msgLen := length + 3
	prefix := []byte{byte(msgLen & 0xFF), byte(msgLen >> 8), client_prot.CMD_MESSAGE}
	msg := string(prefix) + string(p[:length])
	up.writeBlocking_Bl([]byte(msg))
	return len(p), nil
}

func (up *user) AddScore(owner uint32, points float64) {
	score.Add(owner, points)
}

func (up *user) Teleport(b []byte) {
	xLSB := b[0]
	yLSB := b[1]
	zLSB := b[2]
	coord := up.pl.coord.GetChunkCoord().UpdateLSB(xLSB, yLSB, zLSB)
	x, y, z, ok := superChunkManager.GetTeleport(&coord)
	if !ok {
		up.Printf_Bl("#FAIL")
	} else {
		var uc user_coord
		uc.X = float64(coord.X)*CHUNK_SIZE + float64(x)
		uc.Y = float64(coord.Y)*CHUNK_SIZE + float64(y)
		uc.Z = float64(coord.Z)*CHUNK_SIZE + float64(z)
		if req := MonsterDifficulty(&uc); req > up.pl.level {
			up.Printf_Bl("#FAIL Level %d required", req)
		} else {
			f1 := func(other *user) {
				log.Println("Plopp to", other.pl.name)
				other.Printf_Bl("#PLP1")
				other.Lock()
				other.SomeoneMoved(up)
				other.Unlock()
			}
			up.pl.coord.CallNearPlayers_RLq(f1, up)
			up.pl.coord = uc
			f2 := func(other *user) {
				log.Println("Boom to", other.pl.name)
				other.Printf_Bl("#BOOM")
				other.Lock()
				other.SomeoneMoved(up)
				other.Unlock()
			}
			uc.CallNearPlayers_RLq(f2, up)
			up.Printf_Bl("#BOOM")
		}
	}
}
