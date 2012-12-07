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

//
// Listen for incoming messages from a client, decode the protocol and call the appropriate
// function, usually in manage_clients.go. All data going back to the client also pass through
// here. There is one goroutine spawned for every client.
//

import (
	"net"
	// "fmt"
	"chunkdb"
	. "client_prot"
	"log"
	"os"
	"time"
)

var (
	// Just for nice info, keep track of who was last logged in
	lastUser     string
	timeOfLogout time.Time
)

const (
	dummyLoginName = "<login>"
)

// This is a function that only listens for new connections. The listening is done forever in a goroutine of its own,
// while this function returns the success status.
func SetupListenForClients_WLuBlWLqWLa(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		// Errors are not expected from the call to accept. If they happen anyway, log a message and give it up.
		for failures := 0; failures < 100; {
			conn, err := listener.Accept()
			if err != nil {
				log.Print("Failed listening: ", err, "\n")
				failures++
			}
			if ok, index := NewClientConnection_WLa(conn); ok {
				// A new connection is established. Spawn a new gorouting to handle that player
				go ManageOneClient_WLuBlWLqWLa(conn, index)
			}
		}
		log.Println("Too many listener.Accept() errors, giving up")
		os.Exit(1)
	}()
	return nil
}

// Send the protocol version to the client.
func SendProtocolVersion_Bl(conn net.Conn) {
	b := []byte{11, 0, CMD_PROT_VERSION,
		ProtVersionMinor & 0xFF,
		ProtVersionMinor >> 8,
		ProtVersionMajor & 0xFF,
		ProtVersionMajor >> 8,
		byte(ClientCurrentMinorVersion & 0xFF),
		byte(ClientCurrentMinorVersion >> 8),
		byte(ClientCurrentMajorVersion & 0xFF),
		byte(ClientCurrentMajorVersion >> 8),
	}
	conn.Write(b)
}

// This is executed as one process for each client
func ManageOneClient_WLuBlWLqWLa(conn net.Conn, i int) {
	// log.Print("RemoteAddr ", conn.RemoteAddr(), "\n")
	SendProtocolVersion_Bl(conn)
	ManageOneClient2_WLuWLqWLmBlWLcWLw(conn, i)
	if !NameIsTestPlayer(allPlayers[i].Name) && allPlayers[i].Name != dummyLoginName {
		lastUser = allPlayers[i].Name
		timeOfLogout = time.Now()
	}
	CmdClose_BlWLqWLuWLa(i)
}

// This is executed as one process for each client. All messages sent because of this process must use blocking send.
// The reason is that non-blocking send will be sent in a channel to this process, and we must not send messages
// to our own channel.
func ManageOneClient2_WLuWLqWLmBlWLcWLw(conn net.Conn, i int) {
	buff := make([]byte, 50) // Command buffer, also used for blocking messages.
	up := allPlayers[i]
	up.Name = dummyLoginName // To have something to print
	previous := time.Now()
	longPrevious := previous
	previousAttack := previous
	// Loop until player disconnects
	for {
		// Measure how much time has passed since last iteration
		now := time.Now()
		delta := now.Sub(previous)
		if up.connState == PlayerConnStateIn {
			// Ignore this unless the player is logged in.
			if delta > ObjectsUpdatePeriod {
				// These functions cost a lot, don't run them unless enough time has passed.
				previous = now
				// Check if near objects have moved and send messages to the client
				clientTellMovedObjects_Bl(up)
				// Update new position for player
				up.cmdUpdatePosition_WLuWLqWLmWLwWLc()
				fullReport := false
				if now.Sub(longPrevious) > 2*time.Second {
					fullReport = true
					longPrevious = now
				}
				// Tell everyone near if the player moved
				up.checkOnePlayerPosChanged_RLuWLqBl(fullReport)
			}
			delta = now.Sub(previousAttack)
			if delta > CnfgAttackPeriod {
				previousAttack = now
				up.ManageAttackPeriod_WLuBl(delta) // Manage general attacking tasks
			}
		}
		if up.updatedStats {
			// As the player isn't locked, the stats may be updated while this is done. Clear the flag
			// first, to ensure other updates are not lost. The transient flags are cleared later, and so it may happen
			// that a flag will be lost. But this is not vital information, so a loss can be accepted if it is unlikely.
			// log.Printf("State %v, flags 0x%x, hp %v, exp %v, level %v, mana %v\n", up.connState, up.flags, up.HitPoints, up.Exp, up.Level, up.Mana)
			up.updatedStats = false
			up.SendMsgUpdatedStats_Bl(buff)
			up.flags &= ^UserFlagTransientMask // Clear all transient flags, now that the client has been informed.
		}
		if up.forceSave {
			up.forceSave = false
			CmdSavePlayerNow_RluBl(i)
		}
		// Read out all waiting data, if any, from the incoming channels
		for moreData := true; moreData; {
			select {
			case clientMessage := <-up.channel:
				up.writeBlocking_Bl(clientMessage)
			case clientCommand := <-up.commandChannel:
				clientCommand(up)
			default:
				moreData = false
			}
			if up.connState == PlayerConnStateDisc {
				CmdSavePlayerNow_RluBl(i) // Force a save.
				return
			}
		}
		// Set a new deadline.
		conn.SetReadDeadline(time.Now().Add(ObjectsUpdatePeriod))
		n, err := conn.Read(buff[0:2]) // Read the length information. This will block for ObjectsUpdatePeriod ns
		if err != nil {
			if e2, ok := err.(*net.OpError); ok && (e2.Timeout() || e2.Temporary()) {
				// log.Printf("Read timeout %v", e2) // This will happen frequently
				continue
			}
			if *verboseFlag > 1 {
				// This is a normal case
				log.Printf("Disconnect %v because of '%v'\n", up.Name, err)
			}
			CmdSavePlayerNow_RluBl(i) // Force a save, so as not to lose anything, Ignore result.
			return
		}
		if n == 1 {
			if *verboseFlag > 1 {
				log.Printf("Got %d bytes reading from socket\n", n)
			}
			for n == 1 {
				n2, err := conn.Read(buff[1:2]) // Second byte of the length
				if err != nil || n2 != 1 {
					if e2, ok := err.(*net.OpError); ok && (e2.Timeout() || e2.Temporary()) {
						log.Printf("Read timeout %v", e2)
						continue
					}
					log.Printf("Failed again to read: %v\n", err)
					return
				}
				n = 2
			}
		}
		if n != 2 {
			log.Printf("Got %d bytes readin from socket\n", n)
			continue
		}
		length := int(uint(buff[1])<<8 + uint(buff[0]))
		if length > cap(buff) {
			buff2 := make([]byte, length)
			copy(buff2, buff[0:2])
			buff = buff2
			if *verboseFlag > 2 {
				log.Printf("Expecting %d bytes, which is too much for buffer. BUffer was extended\n", length)
			}
		}
		// Read the rest of the bytes
		for n2 := n; n2 < length; {
			// If unlucky, we only get one byte at a time
			var e2 net.Error
			for {
				// Loop here until we get what we want.
				n, err = conn.Read(buff[n2:length])
				if err == nil {
					break // No error
				}
				e2, ok := err.(net.Error)
				if ok && !e2.Temporary() && !e2.Timeout() {
					break // Bad error, can't handle it.
				}
			}
			if err != nil {
				if *verboseFlag > 0 {
					log.Printf("Disconnect %v because of '%v'.\n", up.Name, err)
					if e2 != nil {
						log.Printf("Temporary: %v, Timeout: %v\n", e2.Temporary(), e2.Timeout())
					}
				}
				CmdSavePlayerNow_RluBl(i) // Force a save, so as not to lose anything, Ignore result.
				return
			}
			n2 += n
		}
		trafficStatistics.AddReceived(length)
		// fmt.Printf("ManageOneClient: command (%d bytes) %v\n", n, buff[:length])
		switch buff[2] {
		case CMD_PING:
			if buff[3] == 0 {
				// Request message
				buff[3] = 1                               // Change it to response type
				allPlayers[i].writeBlocking_Bl(buff[0:4]) // And send it back.
			}
		case CMD_SAVE:
			CmdSavePlayerNow_RluBl(i)
		case CMD_LOGIN:
			if length <= 3 {
				// TODO. Too short command, no login name
				log.Print("LOGIN no name\n")
				return
			}
			if *verboseFlag > 2 {
				log.Printf("Logincmd %v\n", buff[3:length])
			}
			up.CmdLogin_WLwWLuWLqBlWLc(string(buff[3:length]))
			if *verboseFlag > 1 {
				defer log.Printf("Leaving player '%v'\n", up.Name)
			}
		case CMD_RESP_PASSWORD:
			// log.Printf("Logincmd %v\n", buff[3 : length])
			if !up.CmdPassword_WLwWLuWLqBlWLc(buff[3:length]) {
				buff[0] = 3
				buff[1] = 0
				buff[2] = CMD_LOGINFAILED
				allPlayers[i].writeBlocking_Bl(buff[0:3]) // Tell client login failed.
				if *verboseFlag > 0 {
					log.Printf("Disconnect %v\n", up.Name)
				}
				return
			}
			up.FileMessage(*welcomeMsgFile)
			if len(allPlayerIdMap) > 1 {
				up.Printf_Bl("Current players:")
				up.ReportPlayers()
			} else if lastUser != "" {
				up.Printf_Bl("Last logout: %s in %s", lastUser, time.Now().Sub(timeOfLogout))
			}
		case CMD_QUIT:
			CmdSavePlayerNow_RluBl(i)
			if *verboseFlag > 1 {
				log.Printf("Disconnect %v\n", up.Name)
			}
			return
		case CMD_GET_COORDINATE:
			up.CmdReportCoordinate_RLuBl(false)
		case CMD_ATTACK_MONSTER:
			if length != 7 {
				log.Printf("CMD_ATTACK_MONSTER illegal length: %v\n", buff[0:length])
				return
			}
			up.CmdAttackMonster_WLuRLm(buff[3:7])
		case CMD_PLAYER_ACTION:
			if length != 4 {
				log.Printf("CMD_PLAYER_ACTION illegal length: %v\n", buff[0:length])
				return
			}
			up.CmdPlayerAction_WLuBl(buff[3])
		case CMD_READ_CHUNK:
			if length != 15 {
				log.Printf("CommandReadChunk illegal length: %v\n", buff[0:length])
				return
			} else {
				MakeCommandReadChunk_WLwWLcBl(i, buff[3:length])
			}
		case CMD_VRFY_CHUNCK_CS:
			if length < 10 || ((length-3)%7 != 0) {
				// fmt.Printf("Verify Checksum length error!")
				log.Printf("CMD_VRFY_CHUNCK_CS illegal length: %v\n", buff[0:length])
			} else {
				// A list of chunk checksums can be recieved, these should be verified and if the
				// checksum is not correct, the updated block should be sent
				// fmt.Printf("CMD_VRFY_CHUNCK_CS - %v %v %v %v\n", buff[6], buff[7], buff[8], buff[9] )
				CommandVerifyChunkCS_WLwWLcBl(i, buff[3:length])
			}
		case CMD_HIT_BLOCK:
			if length != 18 {
				log.Printf("HitBlockCommand illegal length %d: %v\n", length, buff[0:length])
				return
			} else {
				var b []byte
				var cc chunkdb.CC
				cc.X, b, _ = ParseInt32(buff[3:length])
				cc.Y, b, _ = ParseInt32(b)
				cc.Z, b, _ = ParseInt32(b)
				up.HitBlock_WLwWLcRLq(cc, b[0], b[1], b[2])
				// fmt.Printf("ManageOneClient %v\n", hbc)
				// fmt.Println("Buffer ", buff[0:18])
			}
		case CMD_BLOCK_UPDATE:
			if length != 19 {
				// The block update command allows for many blocks to be updated at the same time, but
				// that is for the server->client, not for the client->server.
				log.Printf("AttachBlockCommand illegal length %d: %v\n", length, buff[0:length])
				return
			} else {
				var b []byte
				var cc chunkdb.CC
				cc.X, b, _ = ParseInt32(buff[3:length])
				cc.Y, b, _ = ParseInt32(b)
				cc.Z, b, _ = ParseInt32(b)
				bl := block(b[3])
				if bl == BT_Teleport {
					cp := ChunkFind_WLwWLc(cc)
					cp.SetTeleport(cc, up, b[0], b[1], b[2])
				} else {
					// log.Printf("Attach block %v at chunk %v\n", bl, cc)
					CmdAttachBlock_WLwWLcRLq(cc, b[0], b[1], b[2], bl, i)
				}
			}
		case CMD_JUMP:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_JUMP)
		case CMD_START_FWD:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_START_FWD)
		case CMD_STOP_FWD:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_STOP_FWD)
		case CMD_START_BWD:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_START_BWD)
		case CMD_STOP_BWD:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_STOP_BWD)
		case CMD_START_LFT:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_START_LFT)
		case CMD_STOP_LFT:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_STOP_LFT)
		case CMD_START_RGT:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_START_RGT)
		case CMD_STOP_RGT:
			up.CmdPlayerMove_WLuWLqWLmWLwWLc(CMD_STOP_RGT)
		case CMD_SET_DIR:
			if length != 7 {
				log.Printf("Illegal SET_DIR client command: %v\n", buff[0:length])
				return
			} else {
				MakeDirectionsCommand(i, buff[3:length])
			}
		case CMD_DEBUG:
			up.playerStringMessage_RLuWLwRLqBlWLaWLc(buff[3:length])
		case CMD_USE_ITEM:
			code := ObjectCode(buff[3:7])
			lvl := uint32(0)
			if length == 11 {
				// This is the new proper way, where a level of the item is also provided,
				// but some clients remains with the old format. TODO: Clean up.
				lvl, _, _ = ParseUint32(buff[7:11])
			}
			up.Inventory.Use_WluBl(up, code, lvl)
		case CMD_DROP_ITEM:
			code := ObjectCode(buff[3:7])
			lvl, _, _ := ParseUint32(buff[7:11])
			up.Lock()
			// The Use function will not really do anything, only return a function. That way, only a read lock is needed.
			// The reason for this is that the Use function will do callbacks that will, in turn, lock what is needed. As this is
			// not known now, except that we know the user has to be read locked.
			val := ItemValueAsDrop(up.Level, lvl, code) * CnfgItemRewardNormalizer
			if val >= 0 {
				up.Inventory.Remove(code, lvl)
				up.AddExperience(val)
			}
			up.Unlock()
			// log.Println("CMD_DROP_ITEM", code, lvl, val)
			ReportOneInventoryItem_WluBl(up, code, lvl)
		case CMD_REQ_PLAYER_INFO:
			uid, _, _ := ParseUint32(buff[3:7])
			allPlayersSem.RLock()
			up, ok := allPlayerIdMap[uid]
			allPlayersSem.RUnlock()
			if ok {
				l := len(up.Name)
				buff[0] = byte(8 + l)
				buff[1] = 0
				buff[2] = CMD_RESP_PLAYER_NAME
				EncodeUint32(uid, buff[3:7])
				buff[7] = up.AdminLevel
				copy(buff[8:], up.Name)
				allPlayers[i].writeBlocking_Bl(buff[0 : 8+l])
				allPlayers[i].ReportEquipment_Bl(up)
			}
		case CMD_VRFY_SUPERCHUNCK_CS:
			if length < 10 || ((length-3)%7 != 0) {
				// fmt.Printf("Verify Checksum length error!")
				log.Printf("CMD_VRFY_CHUNK_CS illegal length: %v\n", buff[0:length])
			} else {
				// A list of chunk checksums can be recieved, these should be verified and if the
				// checksum is not correct, the updated block should be sent
				// fmt.Printf("CMD_VRFY_CHUNK_CS - %v %v %v %v\n", buff[6], buff[7], buff[8], buff[9] )
				up.CommandVerifySuperchunkCS_Bl(buff[3:length])
			}
		case CMD_TELEPORT:
			up.Teleport(buff[3:length])
		default:
			log.Printf("Unknown command '%v'.\n", buff[0:length])
			return
		}
		if length < n {
			// TODO: There are more commands still in the data, go to next
			panic("Too much")
		}
	}
}

func MakeCommandReadChunk_WLwWLcBl(i int, b []byte) {
	// fmt.Printf("MakeCommandReadChunk index %v argument %v\n", i, b)
	x, b, _ := ParseUint32(b)
	y, b, _ := ParseUint32(b)
	z, b, _ := ParseUint32(b)
	up := allPlayers[i]
	up.CmdReadChunk_WLwWLcBl(chunkdb.CC{int32(x), int32(y), int32(z)})
}

func MakeDirectionsCommand(i int, b []byte) {
	hdir, b2, ok := ParseUint16(b)
	if !ok {
		log.Printf("MakeDirectionsCommand: Bad hor dir in %v\n", b)
		return
	}
	vdir, _, ok := ParseUint16(b2)
	if !ok {
		log.Printf("MakeDirectionsCommand: Bad vert dir in %v\n", b)
		return
	}
	dirHor := float32(hdir) / 100.0
	dirVert := float32(int16(vdir)) / 100.0
	CmdSetDirections(i, dirHor, dirVert)
}

func (up *user) ManageAttackPeriod_WLuBl(delta time.Duration) {
	mp := up.aggro
	dist2 := float64(0) // Distance to monster, squared
	if mp != nil {
		dx := up.Coord.X - mp.Coord.X
		dy := up.Coord.Y - mp.Coord.Y
		dz := up.Coord.Z - mp.Coord.Z
		dist2 = dx*dx + dy*dy + dz*dz
		if dist2 > CnfgMonsterAggroDistance*CnfgMonsterAggroDistance {
			mp = nil
			up.Printf_Bl("Too far away for combat")
		}
		if mp == nil || mp.invalid || mp.HitPoints <= 0 || mp.dead || up.Dead {
			up.Lock()
			up.aggro = nil
			up.flags &= ^UserFlagInFight
			up.Unlock()
			up.updatedStats = true
		} else {
			if dist2 <= CnfgMeleeDistLimit*CnfgMeleeDistLimit {
				// Close enough to hit
				mp.Hit_WLuBl(up, 1)
			}
		}
	} else if !up.Dead && up.aggro == nil && (up.HitPoints != 1 || up.Mana != 1) {
		// Restore health and mana for player if not being dead and not attacking
		up.HealthAndMana_WLu(delta)
	}
}

// Heal the player and restore mana, depending on how much time has passed.
func (up *user) HealthAndMana_WLu(delta time.Duration) {
	up.Lock()
	newHP := up.HitPoints + float32(delta)/CnfgHealingPeriod
	if newHP > 1 {
		newHP = 1
	}
	if newHP > up.HitPoints {
		up.HitPoints = newHP
		up.updatedStats = true
	}
	up.Unlock()
	// The mana is only controlled by the ManageOneClient2() function (which called this function), and so there is no need for a lock.
	newMana := up.Mana + float32(delta)/CnfgHealingPeriod
	if newMana > 1 {
		newMana = 1
	}
	if newMana > up.Mana {
		up.Mana = newMana
		up.updatedStats = true
	}
}
