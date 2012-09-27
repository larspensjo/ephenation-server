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
// This is an automated test suite
// TODO: Eventually, this should be compiled separately instead of having it permanently in the source code

import (
	"bytes"
	"chunkdb"
	"client_prot"
	"crypto/rc4"
	"ephenationdb"
	"fmt"
	"keys"
	"license"
	"math"
	"quadtree"
	"simplexnoise"
	"time"
	"twof"
)

var (
	testCount   int
	testFailed  int
	testSuccess int
)

func DoTest() {
	fmt.Printf("Ephenation starting automatic testing\n")
	*allowTestUser = true // Override this flag
	DoTestChunkSaveRestore()
	DoTestTriggerBlocks_WLwWLc() // Do this early on, as a fake chunk will be used
	DoTestTextActivators()
	DoTestActivatorConditions()
	DoTestEncoding()
	DoTestChunkCompression()
	DoTestQuadtree_WLq()
	DoTestMonsterSpawnAndPurge_WLuWLqBlWLwWLaWLmWLc()
	DoTestEncrypt()
	dbok := DoTestSQL()
	if dbok {
		DoTestLicense_Bl()
	}
	// DoTestSimplexNoise()
	DoTestPlayerManagement_WLuWLqWLmBlWLaWLwWLc()
	DoTestCoordinates()
	if dbok {
		DoTestChunkdb_WLwBlWLc()
	}
	DoTestCombat_WLuBl()
	DoTestFriends_WLaWLwWLuWLqBlWLc()
	DoTestKeyRing()
	DoTestJellyBlocks()
	fmt.Printf("Tests done. %d tests (%d successful + %d failures)\n", testCount, testSuccess, testFailed)
}

func DoTestCheck(what string, ok bool) {
	testCount++
	if ok {
		testSuccess++
		if *verboseFlag > 1 {
			fmt.Println("Successful test ", what)
		}
	} else {
		testFailed++
		fmt.Print("Failed test ", what, "\n")
	}
}

func DoTestEncoding() {
	{
		var x1 uint32 = 0x12345678
		var b1 [4]byte
		EncodeUint32(x1, b1[:])
		x2, b2, ok := ParseUint32(b1[:])
		DoTestCheck("EncodeUint32/ParseUint32 bool ok", ok)
		DoTestCheck("EncodeUint32/ParseUint32 equal", x1 == x2)
		DoTestCheck("EncodeUint32/ParseUint32 slice updated", len(b2) == 0)
	}

	{
		var x1 uint64 = 0x1122334455667788
		var b1 [8]byte
		EncodeUint64(x1, b1[:])
		x2, b2, ok := ParseUint64(b1[:])
		DoTestCheck("EncodeUint64/ParseUint64 bool ok", ok)
		DoTestCheck("EncodeUint64/ParseUint64 equal", x1 == x2)
		DoTestCheck("EncodeUint64/ParseUint64 slice updated", len(b2) == 0)
	}
}

func DoTestChunkCompression() {
	coord := chunkdb.CC{0, 0, 4}
	ch := dBCreateChunk(coord)
	ch.ch_comp = nil // Make sure there is no compress data yet
	rc := raw_chunk{}
	// Copy the raw chunk
	for x := uint(0); x < CHUNK_SIZE; x++ {
		for y := uint(0); y < CHUNK_SIZE; y++ {
			for z := uint8(0); z < CHUNK_SIZE; z++ {
				rc[x][y][z] = ch.rc[x][y][z]
			}
		}
	}
	ch.compress()
	ch.rc = nil
	ch.rc = decompressChunk(ch.ch_comp)
	// Verify that the content is the same
	success := true
	for x := uint(0); x < CHUNK_SIZE; x++ {
		for y := uint(0); y < CHUNK_SIZE; y++ {
			for z := uint8(0); z < CHUNK_SIZE; z++ {
				if rc[x][y][z] != ch.rc[x][y][z] {
					success = false
				}
			}
		}
	}
	DoTestCheck("Chunk compress/decompress", success)
}

// Define an object used for testing the OcTree
type testObj twof.TwoF

func (o *testObj) GetPreviousPos() *twof.TwoF { return (*twof.TwoF)(o) } // Always the same as objects are not moved
func (*testObj) GetId() uint32                { return 0 }               // Not needed for testing
func (*testObj) GetType() uint8               { return 0 }               // Not needed for testing
func (*testObj) GetZ() float64                { return 0 }               // Not needed for testing
func (*testObj) GetDir() float32              { return 0 }               // Not needed for testing

func DoTestQuadtree_WLq() {
	t := quadtree.MakeQuadtree(twof.TwoF{0.0, 0.0}, twof.TwoF{2.0, 2.0}, 1)
	var objList []*testObj
	const (
		testNumber = 20
	)
	for i := 0; i < testNumber; i++ {
		objList = append(objList, &testObj{float64(i) / 10, 0.1})
	}
	for _, o := range objList {
		// fmt.Printf("Add object %p %v\n", o, o)
		t.Add_WLq(o, o.GetPreviousPos())
	}
	near := t.FindNearObjects_RLq(&twof.TwoF{0.4, 0.1}, 0.201) // Find 0.2, 0.3, 0.4, 0.5 and 0.6
	DoTestCheck("FindNearObjects", len(near) == 5)
	// fmt.Printf("Tree after adding:\n%v\n", t)
	for _, o := range objList {
		// fmt.Printf("Remove object %p %v\n", o, o)
		t.Remove_WLq(o)
	}
	// fmt.Printf("Tree after removing again\n%v\n", t)
	DoTestCheck("Quadtree empty", t.Empty())
}

// The random spawn of monsters will fail, as the geography around the player makes the spawning go wrong.
// The test need to be improved.
func DoTestMonsterSpawnAndPurge_WLuWLqBlWLwWLaWLmWLc() {
	DoTestCheck("DoTestMonsterSpawnAndPurge player quad tree empty", playerQuadtree.Empty())
	// Create a dummy user.
	conn := MakeDummyConn()
	_, index := NewClientConnection_WLa(conn)
	DoTestCheck("DoTestMonsterSpawnAndPurge: create one player", numPlayers == 1)
	DoTestCheck("DoTestMonsterSpawnAndPurge: login nack", !conn.TestCommandSeen(client_prot.CMD_LOGIN_ACK))
	CmdLogin_WLwWLuWLqBlWLc("test0", index)                                                                          // login name is an email, but don't care for this test.
	DoTestCheck("DoTestMonsterSpawnAndPurge: request password", !conn.TestCommandSeen(client_prot.CMD_REQ_PASSWORD)) // No password for test users
	up := allPlayers[0]
	DoTestCheck("DoTestMonsterSpawnAndPurge: login ack", conn.TestCommandSeen(client_prot.CMD_LOGIN_ACK))
	DoTestCheck("DoTestMonsterSpawnAndPurge: expect no license", up.lic == nil)
	// fmt.Printf("%#v\n", up)
	for i := 0; i < MonsterLimitForRespawn; i++ {
		addMonsterToPlayer_WLwWLuWLqWLmWLc(up)
	}
	actual := CountNearMonsters_RLq(up.GetPreviousPos())
	fmt.Println("DoTestMonsterSpawnAndPurge: Num spawned: ", actual)
	DoTestCheck("DoTestMonsterSpawnAndPurge: Spawn monsters", actual == MonsterLimitForRespawn)
	addMonsterToPlayer_WLwWLuWLqWLmWLc(up) // One too many, shall be ignored.
	actual = CountNearMonsters_RLq(up.GetPreviousPos())
	DoTestCheck("DoTestMonsterSpawnAndPurge: Spawn monsters (too many)", actual == MonsterLimitForRespawn)
	DoTestCheck("DoTestMonsterSpawnAndPurge: spawn monsters 2:nd check", len(monsterData.m) == actual)
	DoTestCheck("DoTestMonsterSpawnAndPurge: no obj list yet", !conn.TestCommandSeen(client_prot.CMD_OBJECT_LIST))
	clientTellMovedObjects_Bl(up)
	DoTestCheck("DoTestMonsterSpawnAndPurge: obj list", conn.TestCommandSeen(client_prot.CMD_OBJECT_LIST))
	// fmt.Printf("DoTestMonsterSpawnAndPurge: Monster data %#v\n", monsterData.m)

	// Remove the player and check that monsters are purged.
	CmdClose_BlWLqWLuWLa(index)
	DoTestCheck("DoTestMonsterSpawnAndPurge: conn closed", !conn.TestOpen())
	DoTestCheck("DoTestMonsterSpawnAndPurge: remove one player", numPlayers == 0)
	CmdPurgeMonsters_WLmWLq()
	DoTestCheck("DoTestMonsterSpawnAndPurge: monsters purged", len(monsterData.m) == 0)
}

// Verify functionality of the rc4 encryption. Not that I don't trust it, but I need to test
// that I use it the right way.
func DoTestEncrypt() {
	type s struct {
		key, inp, outp string
	}
	tstVect := []s{
		// http://en.wikipedia.org/wiki/RC4#Test_vectors
		{"Key", "Plaintext", "BBF316E8D940AF0AD3"},
		{"Wiki", "pedia", "1021BF0420"},
		{"Secret", "Attack at dawn", "45A01F645FC35B383552544B9BF5"},
	}
	for _, tst := range tstVect {
		dst := make([]byte, len(tst.inp))
		cipher, err := rc4.NewCipher([]byte(tst.key))
		DoTestCheck("DoTestEncrypt new cipher", err == nil)
		cipher.XORKeyStream(dst, []byte(tst.inp))
		eq := true
		for i, ch := range dst {
			if fmt.Sprintf("%02X", ch) != string(tst.outp[i*2:i*2+2]) {
				eq = false
			}
		}
		DoTestCheck("DoTestEncrypt check encryption", eq)
	}
}

func DoTestLicense_Bl() {
	const (
		email = "a@b"
		name  = "nm"
		pass  = "abcdefABCDEF"
	)
	lic := license.Make(email)
	lic.Names = []string{name}
	lic.License = license.GenerateKey()
	lic.NewPassword(pass)
	if *verboseFlag > 0 {
		fmt.Printf("DoTestLicense load lic1 %#v\n", lic)
	}
	DoTestCheck("DoTestLicense VerifyPassword1", lic.VerifyPassword(pass, encryptionSalt))
	saveSuccess := lic.Save_Bl()
	DoTestCheck("DoTestLicense Save", saveSuccess)
	lic2 := license.Load_Bl(email)
	if *verboseFlag > 0 {
		fmt.Printf("DoTestLicense load lic2 %#v\n", lic2)
	}
	DoTestCheck("DoTestLicense Load", lic2 != nil)
	if lic2 != nil {
		DoTestCheck("DoTestLicense VerifyPassword2", lic2.VerifyPassword(pass, encryptionSalt))
	}
}

// Verify correct connectivity between triggers and activators.
func DoTestTriggerBlocks_WLwWLc() {
	msg1 := "apa"
	ch := dBCreateChunk(chunkdb.CC{0, 0, 0})
	ch.ComputeLinks()
	DoTestCheck("DoTestTriggerBlocks No initial links", len(ch.blTriggers) == 0 && len(ch.triggerMsgs) == 0)
	// Define a trigger, an activator, and a link in between. Easy one, not at the
	// border to another chunk
	ch.rc[5][5][5] = BT_Trigger
	ch.rc[5][5][6] = BT_Text
	ch.ComputeLinks()
	DoTestCheck("DoTestTriggerBlocks zero links", len(ch.blTriggers) == 1 && len(ch.triggerMsgs) == 1)
	msgp := ch.FindActivator(5, 5, 6)
	*msgp = []string{msg1}
	DoTestCheck("DoTestTriggerBlocks zero links add msg", len(ch.triggerMsgs[0].Message) == 1 && ch.triggerMsgs[0].Message[0] == msg1)
	ch.ComputeLinks() // Compute the same links again, and make sure message is copied
	DoTestCheck("DoTestTriggerBlocks zero links same msg", len(ch.triggerMsgs[0].Message) == 1 && ch.triggerMsgs[0].Message[0] == msg1)
	ch.rc[4][5][5], ch.rc[5][4][5], ch.rc[5][5][4], ch.rc[6][5][5], ch.rc[5][6][5], ch.rc[5][5][6] = BT_Text, BT_Text, BT_Text, BT_Text, BT_Text, BT_Text
	ch.ComputeLinks()
	// fmt.Println(ch.triggerMsgs)
	DoTestCheck("DoTestTriggerBlocks zero links 6 spawners", len(ch.blTriggers) == 6 && len(ch.triggerMsgs) == 6)
	ch.rc[4][5][5], ch.rc[5][4][5], ch.rc[5][5][4], ch.rc[6][5][5], ch.rc[5][6][5], ch.rc[5][5][6] = BT_Air, BT_Air, BT_Air, BT_Air, BT_Air, BT_Air
	ch.rc[5][5][6] = BT_Link
	ch.rc[5][5][7] = BT_Text
	ch.ComputeLinks()
	DoTestCheck("DoTestTriggerBlocks One link", len(ch.blTriggers) == 1 && len(ch.triggerMsgs) == 1)
	ch.rc[5][6][5] = BT_Link
	ch.rc[5][7][5] = BT_Link
	ch.rc[5][8][5] = BT_Link
	ch.rc[5][9][5] = BT_Text
	ch.ComputeLinks()
	DoTestCheck("DoTestTriggerBlocks Two links", len(ch.blTriggers) == 2 && len(ch.triggerMsgs) == 2)
	ch.rc[5][6][6] = BT_DeTrigger // Connect to both spawners
	ch.ComputeLinks()
	DoTestCheck("DoTestTriggerBlocks DeTrigger also", len(ch.blTriggers) == 4 && len(ch.triggerMsgs) == 2) // Two triggers times two activators
	for _, trig := range ch.blTriggers {
		DoTestCheck("DoTestTriggerBlocks triggers always connected to messages", trig.msg != nil)
	}

	// Add a message before the save
	msgp = ch.FindActivator(5, 9, 5)
	DoTestCheck("DoTestTriggerBlocks Add message again", msgp != nil)
	*msgp = []string{msg1}
	m1 := ch.triggerMsgs[0]
	m2 := ch.triggerMsgs[1]
	m1ok, m2ok := true, true
	if m1.X == 5 && m1.Y == 5 && m1.Z == 7 && m1.Message != nil {
		m1ok = false
	}
	if m2.X == 5 && m2.Y == 5 && m2.Z == 7 && m2.Message != nil {
		m2ok = false
	}
	if m1.X == 5 && m1.Y == 9 && m1.Z == 5 && m1.Message == nil {
		m1ok = false
	}
	if m2.X == 5 && m2.Y == 9 && m2.Z == 5 && m2.Message == nil {
		m2ok = false
	}
	DoTestCheck("DoTestTriggerBlocks message on right activator", m1ok && m2ok)

	// Test that save and restore of this chunk will also restore the activator messages
	ch.compress() // Update the compressed data
	var buf bytes.Buffer
	ok := ch.WriteFS(&buf)
	// fmt.Println("DoTestTriggerBlocks chunk saved", buf.Bytes())
	DoTestCheck("DoTestTriggerBlocks Write", ok)
	if !ok {
		return
	}
	ch2 := dBReadChunk(ch.coord, &buf, int64(buf.Len()))
	DoTestCheck("DoTestTriggerBlocks equal", DoTestChunkCompare(ch, ch2))
	DoTestCheck("DoTestTriggerBlocks DeTrigger Read", len(ch2.blTriggers) == 4 && len(ch2.triggerMsgs) == 2) // Two triggers times two activators
	for _, trig := range ch.blTriggers {
		DoTestCheck("DoTestTriggerBlocks triggers always connected to messages", trig.msg != nil)
	}

	// Check message after the load
	m1 = ch2.triggerMsgs[0]
	m2 = ch2.triggerMsgs[1]
	if m1.X == 5 && m1.Y == 5 && m1.Z == 7 && m1.Message != nil {
		m1ok = false
	}
	if m2.X == 5 && m2.Y == 5 && m2.Z == 7 && m2.Message != nil {
		m2ok = false
	}
	if m1.X == 5 && m1.Y == 9 && m1.Z == 5 && m1.Message == nil {
		m1ok = false
	}
	if m2.X == 5 && m2.Y == 9 && m2.Z == 5 && m2.Message == nil {
		m2ok = false
	}
	DoTestCheck("DoTestTriggerBlocks message on right activator after load", m1ok && m2ok)
}

var zeroTime = time.Unix(0, 0)

// Test that the list of activators are handled correctly.
func DoTestTextActivators() {
	const (
		STR1 = "apa"
		STR2 = "bepa"
	)
	var src []textMsgActivator
	var dest []textMsgActivator
	src = append(src, textMsgActivator{5, 5, 5, []string{STR1}, zeroTime})
	dest = append(dest, textMsgActivator{5, 5, 5, nil, zeroTime})
	CopyActivatorListMessages(src, dest)
	DoTestCheck("DoTestTextActivators length test1", len(src) == 1 && len(dest) == 1)
	DoTestCheck("DoTestTextActivators copy1", len(dest[0].Message) == 1 && dest[0].Message[0] == STR1)

	// Add an ativator to src, but not for dest.
	src = append(src, textMsgActivator{5, 5, 6, []string{STR2}, zeroTime})
	CopyActivatorListMessages(src, dest)
	DoTestCheck("DoTestTextActivators length2", len(src) == 2 && len(dest) == 1)
	DoTestCheck("DoTestTextActivators copy2", len(dest[0].Message) == 1 && dest[0].Message[0] == STR1)

	// Add an ativator to dest also.
	dest = append(dest, textMsgActivator{5, 5, 6, nil, zeroTime})
	CopyActivatorListMessages(src, dest)
	DoTestCheck("DoTestTextActivators length3", len(src) == 2 && len(dest) == 2)
	DoTestCheck("DoTestTextActivators copy3", len(dest[1].Message) == 1 && dest[1].Message[0] == STR2)
	DoTestCheck("DoTestTextActivators copy2 still", len(dest[0].Message) == 1 && dest[0].Message[0] == STR1)
}

func DoTestActivatorConditions() {
	var up user
	var ac *user_coord
	up.conn = MakeDummyConn()
	up.pl.level = 9
	up.pl.adminLevel = 5
	inhibit, terminate, _ := up.ActivatorMessage_WLuWLqWLmWLc("X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions default inhibit", inhibit == -1 && terminate == false)
	_, terminate, _ = up.ActivatorMessage_WLuWLqWLmWLc("/level<10 X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions low level test 1", terminate == false)
	_, terminate, _ = up.ActivatorMessage_WLuWLqWLmWLc("/level<9 X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions low level test 2", terminate == true)
	_, terminate, _ = up.ActivatorMessage_WLuWLqWLmWLc("/level>9 X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions high level test 1", terminate == true)
	_, terminate, _ = up.ActivatorMessage_WLuWLqWLmWLc("/level>8 X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions high level test 2", terminate == false)
	_, terminate, _ = up.ActivatorMessage_WLuWLqWLmWLc("/admin>5 X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions admin level test 1", terminate == true)
	_, terminate, _ = up.ActivatorMessage_WLuWLqWLmWLc("/admin>4 X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions admin level test 2", terminate == false)
	_, terminate, _ = up.ActivatorMessage_WLuWLqWLmWLc("/level<10 level>8 X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions level combined 1", terminate == false)
	inhibit, _, _ = up.ActivatorMessage_WLuWLqWLmWLc("/inhibit:9 X", ac, nil, 0)
	DoTestCheck("DoTestActivatorConditions inhibit", inhibit == 9)
}

func DoTestSimplexNoise() {
	var min, max float64

	min, max = 1, -1
	const iter = 1e7
	var sum float64
	for i := 0; i < iter; i++ {
		switch rnd := simplexnoise.Noise1(float64(i) * 1.827231321); {
		case rnd < min:
			min = rnd
			sum += rnd
		case rnd > max:
			max = rnd
			sum += rnd
		default:
			sum += rnd
		}
	}
	// fmt.Printf("DoTestSimplexNoise simplexnoise.Noise1 min %v, max %v, average %v\n", min, max, sum/iter)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise1 min=-1.0", math.Abs(min+1) < 1e-3)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise1 max=1.0", math.Abs(max-1) < 1e-3)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise1 average=0.0", math.Abs(sum/iter) < 0.04) // Not very good, off by more than 3%

	min, max = 1, -1
	sum = 0
	for i := 0; i < iter; i++ {
		switch rnd := simplexnoise.Noise2(float64(i)*1.827411321, float64(i)*7.12312781); {
		case rnd < min:
			min = rnd
			sum += rnd
		case rnd > max:
			max = rnd
			sum += rnd
		default:
			sum += rnd
		}
	}
	// fmt.Printf("DoTestSimplexNoise simplexnoise.Noise2 min %v, max %v, average %v\n", min, max, sum/iter)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise2 min=-1.0", math.Abs(min+1) < 1e-3)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise2 max=1.0", math.Abs(max-1) < 1e-3)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise2 average=0.0", math.Abs(sum/iter) < 1e-4)

	min, max = 1, -1
	sum = 0
	for i := 0; i < iter; i++ {
		switch rnd := simplexnoise.Noise3(float64(i)*1.827131321, float64(i)*7.12372381, float64(i)*4.923716223); {
		case rnd < min:
			min = rnd
			sum += rnd
		case rnd > max:
			max = rnd
			sum += rnd
		default:
			sum += rnd
		}
	}
	// fmt.Printf("DoTestSimplexNoise simplexnoise.Noise3 min %v, max %v, average %v\n", min, max, sum/iter)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise3 min=-1.0", math.Abs(min+1) < 1e-3)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise3 max=1.0", math.Abs(max-1) < 1e-3)
	DoTestCheck("DoTestSimplexNoise simplexnoise.Noise3 average=0.0", math.Abs(sum/iter) < 1e-4)
}

func DoTestPlayerManagement_WLuWLqWLmBlWLaWLwWLc() {
	// Verify that the test player is initialized into a good position in the world.
	const name = "test0"
	_, ok := allPlayerNameMap[name]
	DoTestCheck("DoTestPlayerManagement initial name assoc", ok == false)
	_, ok = allPlayerIdMap[OWNER_TEST]
	DoTestCheck("DoTestPlayerManagement initial uid assoc", ok == false)
	conn := MakeDummyConn()
	ok, index := NewClientConnection_WLa(conn)
	DoTestCheck("DoTestPlayerManagement: create one player", numPlayers == 1 && ok)
	CmdLogin_WLwWLuWLqBlWLc(name, index) // login name is an email, but don't care for this test.

	_, ok = allPlayerNameMap[name]
	DoTestCheck("DoTestPlayerManagement name assoc", ok == true)
	_, ok = allPlayerIdMap[OWNER_TEST]
	DoTestCheck("DoTestPlayerManagement uid assoc", ok == true)

	up := allPlayers[index]
	pl := &up.pl
	DoTestCheck("DoTestPlayerManagement test0 starting point x", pl.coord.X == 0)
	DoTestCheck("DoTestPlayerManagement test0 starting point y", pl.coord.Y == 0)
	DoTestCheck("DoTestPlayerManagement test0 starting point z", pl.coord.Z >= 0)
	DoTestCheck("DoTestPlayerManagement test0 initially stationary", pl.ZSpeed == 0)
	coord := pl.coord
	newZSpeed := UpdateZPos_WLwWLc(1e8, 0, &coord)
	DoTestCheck("DoTestPlayerManagement test0 stationary", coord.Z == pl.coord.Z && newZSpeed == 0)
	// Verify jumping
	up.CmdPlayerMove_WLuWLqWLmWLwWLc(client_prot.CMD_JUMP)
	// println("DoTestPlayerManagement zspeed", pl.ZSpeed)
	for i := 0; i < 10; i++ {
		pl.ZSpeed = UpdateZPos_WLwWLc(1e8, pl.ZSpeed, &pl.coord)
		up.checkOnePlayerPosChanged_RLuWLqBl(false) // Report should be generated that the player moved
		if i == 0 {
			DoTestCheck("DoTestPlayerManagement prev coord updated",
				pl.coord.X == up.prevCoord.X && pl.coord.Y == up.prevCoord.Y && pl.coord.Z == up.prevCoord.Z)
		}
		// println("DoTestPlayerManagement zspeed", pl.ZSpeed)
	}
	// Player should now be at stand still again.
	DoTestCheck("DoTestPlayerManagement test0 stationary after jump", pl.ZSpeed == 0)
	DoTestCheck("DoTestPlayerManagement test0 back to start after jump", pl.coord.Z == coord.Z)
	DoTestCheck("DoTestPlayerManagement player moved Z", conn.TestCommandSeen(client_prot.CMD_REPORT_COORDINATE))
	// Test that the player is maintained correctly in the quadtree
	DoTestCheck("DoTestPlayerManagement quadtree 1 player", !playerQuadtree.Empty())
	// Add dummy players to make the quadtree divide
	var indecies [10]int
	for i := range indecies {
		_, ind := NewClientConnection_WLa(conn)
		indecies[i] = ind
		CmdLogin_WLwWLuWLqBlWLc("test"+fmt.Sprint(ind), ind)
		playerQuadtree.Empty()
	}
	// fmt.Println(playerQuadtree.String())
	pl.coord.X += QuadtreeInitSize / 2          // Enough to move the player to a different section in the quadtree
	up.checkOnePlayerPosChanged_RLuWLqBl(false) // This will update the quadtree
	// fmt.Println(playerQuadtree.String())
	DoTestCheck("DoTestPlayerManagement player moved X", conn.TestCommandSeen(client_prot.CMD_REPORT_COORDINATE))
	CmdClose_BlWLqWLuWLa(index)
	// fmt.Println(playerQuadtree.String())
	for _, i := range indecies {
		CmdClose_BlWLqWLuWLa(i)
	}
	DoTestCheck("DoTestPlayerManagement quadtree 0 players", playerQuadtree.Empty())

	_, ok = allPlayerNameMap[name]
	DoTestCheck("DoTestPlayerManagement final name assoc", ok == false)
	_, ok = allPlayerIdMap[OWNER_TEST]
	DoTestCheck("DoTestPlayerManagement final uid assoc", ok == false)

	// Now test that a disconnected player is updated correctly in the quadtree
	ok, index = NewClientConnection_WLa(conn)
	CmdLogin_WLwWLuWLqBlWLc(name, index) // login name is an email, but don't care for this test.
	DoTestCheck("DoTestPlayerManagement quadtree player back again", ok && !playerQuadtree.Empty())
	allPlayers[index].connState = PlayerConnStateDisc
	CmdClose_BlWLqWLuWLa(index)
	DoTestCheck("DoTestPlayerManagement quadtree disconnected player removed", playerQuadtree.Empty())
}

func DoTestCoordinates() {
	cHigh := chunkdb.CC{0xFF, 0xFF, 0xFF}
	c1 := cHigh.UpdateLSB(1, 2, 3)
	DoTestCheck("DoTestCoordinates chunkdb.CC.UpdateLSB high", c1.X == 0x101 && c1.Y == 0x102 && c1.Z == 0x103)
	cLow := chunkdb.CC{1, 2, 3}
	c2 := cLow.UpdateLSB(0xFF, 0xFE, 0xFD)
	DoTestCheck("DoTestCoordinates chunkdb.CC.UpdateLSB low", c2.X == -1 && c2.Y == -2 && c2.Z == -3)
	// fmt.Println(c1, c2)
}

func DoTestChunkdb_WLwBlWLc() {
	const (
		testName  = "verifying"   // This is a special name allocated in the avatar DB for testing.
		testCoord = math.MaxInt32 // Too far away for anyone to reach
	)
	pl := &player{}
	begin := time.Now()
	uid, ok := pl.Load_WLwBlWLc(testName)
	DoTestCheck("DoTestChunkdb: pl.Load() success", ok)
	delta := time.Now().Sub(begin)
	if *verboseFlag > 0 {
		fmt.Printf("DoTestChunkdb: pl.Load() %d ms\n", delta/1e6)
		fmt.Printf("DoTestChunkdb: Player %#v\n", pl.String())
	}
	terr := pl.territory
	DoTestCheck("DoTestChunkdb: Initial empty territory list", terr == nil)
	chunk1 := chunkdb.CC{testCoord, 0, 0}
	chunk2 := chunkdb.CC{0, testCoord, 0}
	chunk3 := chunkdb.CC{0, 0, testCoord}
	chunkList := []chunkdb.CC{chunk1, chunk2, chunk3}
	ok = chunkdb.SaveAvatar_Bl(uid, chunkList)
	DoTestCheck("DoTestChunkdb: chunkdb.SaveAvatar() success", ok)

	// Load the player again, and verify that the chunk list is updated.
	_, ok = pl.Load_WLwBlWLc(testName)
	DoTestCheck("DoTestChunkdb: second pl.Load() success", ok)
	terr = pl.territory
	DoTestCheck("DoTestChunkdb: Territory list now exists", len(terr) == 3)
	// fmt.Printf("DoTestChunkdb: Returned terr list: %v\n", terr)
	// The returned list is in the opposite order. It is probably incorrect to assume that.
	// TODO: This test usually fail, and then succeed next time!
	DoTestCheck("DoTestChunkdb: Same territory list", terr[0].Equal(chunk3) && terr[1].Equal(chunk2) && terr[2].Equal(chunk1))

	// Clean up and clear the territory list
	ok = chunkdb.SaveAvatar_Bl(uid, nil)
	DoTestCheck("DoTestChunkdb: chunkdb.SaveAvatar(nil) success", ok)
	_, ok = pl.Load_WLwBlWLc(testName)
	DoTestCheck("DoTestChunkdb: third pl.Load() success", ok)
	DoTestCheck("DoTestChunkdb: Final empty territory list", pl.territory == nil)
}

// Some rudimentary tests to get SQL connections
func DoTestSQL() bool {
	db := ephenationdb.New()
	DoTestCheck("DoTestSQL Got connection", db != nil)
	if db == nil {
		// Cancel the other tests
		return false
	}
	ephenationdb.Release(db)
	db2 := ephenationdb.New()
	DoTestCheck("DoTestSQL Got same (cached) connection again", db2 == db)
	db3 := ephenationdb.New()
	DoTestCheck("DoTestSQL Got new connection", db3 != db2 && db3 != nil)
	ephenationdb.Release(db3)
	ephenationdb.Release(db2)
	return true
}

func DoTestCombat_WLuBl() {
	var u user
	var m monster

	u.pl.level = 0
	conn := MakeDummyConn()
	u.conn = conn
	// Test damage from a level 0 player on a monster of various levels.
	DoTestCheck("DoTestCombat_WLu: no hit report", !conn.TestCommandSeen(client_prot.CMD_RESP_PLAYER_HIT_MONSTER))
	for i := uint64(4); i < 32; i++ {
		lvl := uint64(1) << i
		if lvl > math.MaxUint32 {
			break
		}
		m.Level = uint32(lvl)
		m.HitPoints = 1
		u.pl.hitPoints = 1
		m.Hit_WLuBl(&u, 1)
		// fmt.Printf("DoTestCombat %v, new hp: %v\n", lvl, m.HitPoints)
		DoTestCheck("DoTestCombat_WLu No damage on high level monster", m.HitPoints > 0.99)
	}
	DoTestCheck("DoTestCombat_WLu: hit report", conn.TestCommandSeen(client_prot.CMD_RESP_PLAYER_HIT_MONSTER))
}

// Test that friends are added correctly.
func DoTestFriends_WLaWLwWLuWLqBlWLc() {
	const name = "test0"
	conn := MakeDummyConn()
	_, index := NewClientConnection_WLa(conn)
	DoTestCheck("DoTestFriends: create one player", numPlayers == 1 && index == 0)
	CmdLogin_WLwWLuWLqBlWLc(name, index) // login name is an email, but don't care for this test.
	up := allPlayers[index]
	DoTestCheck("DoTestFriends: Empty listener list", len(up.pl.Listeners) == 0)
	DoTestCheck("DoTestFriends: login ack", conn.TestCommandSeen(client_prot.CMD_LOGIN_ACK))
	const (
		ID1 = 1
		ID2 = 2
	)
	var up2 user // Create a new user, that will create friends
	up2.uid = ID1
	notFound, alreadyIn := up2.AddToListener_RLaWLu(name)
	// fmt.Printf("notFound %v, alreadyIn %v\n", notFound, alreadyIn)
	DoTestCheck("DoTestFriends: AddToListener success", notFound == false && alreadyIn == false)
	DoTestCheck("DoTestFriends: One entry listener list", len(up.pl.Listeners) == 1 && up.pl.Listeners[0] == ID1)

	notFound, alreadyIn = up2.AddToListener_RLaWLu(name)
	DoTestCheck("DoTestFriends: AddToListener duplicate", notFound == false && alreadyIn == true)
	DoTestCheck("DoTestFriends: One entry listener list again", len(up.pl.Listeners) == 1 && up.pl.Listeners[0] == ID1)

	up2.uid = ID2 // Now test with the next ID
	notFound, alreadyIn = up2.AddToListener_RLaWLu(name)
	DoTestCheck("DoTestFriends: AddToListener ID2 success", notFound == false && alreadyIn == false)
	DoTestCheck("DoTestFriends: Two entries listener list", len(up.pl.Listeners) == 2 && up.pl.Listeners[1] == ID2)

	notFound, notIn := up2.RemoveFromListener_RLaWLu(name)
	DoTestCheck("DoTestFriends: RemoveFromListener ID2 success", notFound == false && notIn == false)
	DoTestCheck("DoTestFriends: One entry listener list after removal", len(up.pl.Listeners) == 1 && up.pl.Listeners[0] == ID1)

	// Add ID2 back to the list again.
	notFound, alreadyIn = up2.AddToListener_RLaWLu(name)
	DoTestCheck("DoTestFriends: Two entries listener list", len(up.pl.Listeners) == 2 && up.pl.Listeners[1] == ID2)

	// Remove ID1 this time
	up2.uid = ID1
	notFound, notIn = up2.RemoveFromListener_RLaWLu(name)
	DoTestCheck("DoTestFriends: RemoveFromListener ID1 success", notFound == false && notIn == false)
	DoTestCheck("DoTestFriends: One entry listener list after removal", len(up.pl.Listeners) == 1 && up.pl.Listeners[0] == ID2)

	// Remove ID2, which is now the only one
	up2.uid = ID2
	notFound, notIn = up2.RemoveFromListener_RLaWLu(name)
	DoTestCheck("DoTestFriends: RemoveFromListener ID2 success again", notFound == false && notIn == false)
	DoTestCheck("DoTestFriends: Empty listener list after clean-up", len(up.pl.Listeners) == 0)

	// Remove ID2 again, which shall fail
	notFound, notIn = up2.RemoveFromListener_RLaWLu(name)
	DoTestCheck("DoTestFriends: RemoveFromListener ID2 again expect to fail", notFound == false && notIn == true)

	CmdClose_BlWLqWLuWLa(index)
}

func DoTestChunkCompare(ch1, ch2 *chunk) bool {
	equal := ch1.checkSum == ch2.checkSum && ch1.coord.Equal(ch2.coord) && ch1.flag == ch2.flag && ch1.owner == ch2.owner && len(ch1.ch_comp) == len(ch2.ch_comp)
	if !equal {
		return false
	}
	for i, v := range ch1.ch_comp {
		if v != ch2.ch_comp[i] {
			return false
		}
	}
	for x := 0; x < CHUNK_SIZE; x++ {
		for y := 0; y < CHUNK_SIZE; y++ {
			for z := 0; z < CHUNK_SIZE; z++ {
				if ch1.rc[x][y][z] != ch2.rc[x][y][z] {
					return false
				}
			}
		}
	}
	return true
}

// Write a chunk, read it back, and compare that it is the same content
func DoTestChunkSaveRestore() {
	coord := chunkdb.CC{0, 0, 0}
	ch1 := dBCreateChunk(coord)
	var buf bytes.Buffer
	ok := ch1.WriteFS(&buf)
	// fmt.Println("DoTestChunkSaveRestore buffer length", buf.Len())
	DoTestCheck("DoTestChunkSaverestore Write ok", ok)
	ch2 := dBReadChunk(coord, &buf, int64(buf.Len()))
	DoTestCheck("DoTestChunkSaverestore Compare ", DoTestChunkCompare(ch1, ch2))
}

func DoTestKeyRing() {
	var keyRing keys.KeyRing
	const (
		owner = uint32(1)
		kid   = uint(2)
		view  = uint(3)
		name  = "Bronze key"
	)
	DoTestCheck("DoTestKeyRing initial empty", len(keyRing) == 0)
	DoTestCheck("DoTestKeyRing not found", keyRing.Test(owner, kid) == false)
	key1 := keys.Make(owner, kid, name, view)
	keyRing = keyRing.Add(key1)
	DoTestCheck("DoTestKeyRing found after add 1", keyRing.Test(owner, kid))
	key2 := keys.Make(owner+1, kid, name, view)
	keyRing = keyRing.Add(key2)
	DoTestCheck("DoTestKeyRing found again after add 2", keyRing.Test(owner, kid))
	DoTestCheck("DoTestKeyRing found after add 2", keyRing.Test(owner+1, kid))
}

func DoTestJellyBlocks() {
	const (
		testCoord = math.MaxInt32 // Too far away for anyone to reach
	)
	chunk := chunkdb.CC{testCoord, 0, 0}
	ch := dBCreateChunk(chunk)
	// Initialize with something that is not air
	ch.rc[0][0][0] = BT_Stone
	ch.rc[0][0][1] = BT_Stone
	ch.TurnToJelly(0, 0, 0, zeroTime)             // Will timeout immediately
	ch.TurnToJelly(0, 0, 1, time.Now().Add(1e10)) // Timeout in 10s, which is longer than waiting for in the test
	DoTestCheck("DoTestJellyBlocks jelly 1", ch.rc[0][0][0] == BT_Air)
	DoTestCheck("DoTestJellyBlocks jelly 2", ch.rc[0][0][1] == BT_Air)
	DoTestCheck("DoTestJellyBlocks initial list length correct", len(ch.jellyBlocks) == 2)
	ch.RestoreJellyBlocks(false) // Restore only the one with a timeout
	DoTestCheck("DoTestJellyBlocks list decreased", len(ch.jellyBlocks) == 1)
	DoTestCheck("DoTestJellyBlocks jelly 1 reverted", ch.rc[0][0][0] == BT_Stone)
	DoTestCheck("DoTestJellyBlocks jelly 2 remains", ch.rc[0][0][1] == BT_Air)
	ch.RestoreJellyBlocks(true) // Restore all, unconditionally
	DoTestCheck("DoTestJellyBlocks list decreased again", ch.jellyBlocks == nil)
	DoTestCheck("DoTestJellyBlocks jelly 1 still reverted", ch.rc[0][0][0] == BT_Stone)
	DoTestCheck("DoTestJellyBlocks jelly 2 also reverted", ch.rc[0][0][1] == BT_Stone)
}
