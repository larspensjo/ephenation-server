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
// This file contains functions that decode text commands from the client.

import (
	"chunkdb"
	"client_prot"
	"evalsync"
	"fmt"
	"license"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"score"
	"strconv"
	"strings"
	"time"
	"timerstats"
)

// The player sent a string message
func (up *user) playerStringMessage_RLuWLwRLqBlWLaWLc(buff []byte) {
	str := strings.TrimRight(string(buff), " ") // Remove trailing spaces, if any
	if *verboseFlag > 1 {
		log.Printf("User %v cmd: '%v'\n", up.pl.name, str)
	}
	message := strings.SplitN(str, " ", 2)
	switch message[0] {
	case "/keys":
		for _, key := range up.pl.Keys {
			up.Printf_Bl("!%s(%d), uid %d", key.Descr, key.Kid, key.Uid)
		}
	case "/activator":
		if len(message) < 2 {
			return
		}
		up.ActivatorControl(message[1])
	case "/home":
		if len(message) == 1 {
			up.Lock()
			up.pl.coord = up.pl.homeSP
			up.updatedStats = true
			up.Unlock()
		}
	case "/sethome":
		cc := up.pl.coord.GetChunkCoord()
		cp := ChunkFind_WLwWLc(cc)
		if cp.owner != up.uid {
			up.Printf_Bl("#FAIL Not your territory")
			break
		}
		if len(message) == 1 {
			up.Lock()
			up.pl.homeSP = up.pl.coord
			up.Unlock()
			up.Printf_Bl("Home spawn point updated!")
		}
	case "/territory":
		if len(message) < 2 {
			return
		}
		up.TerritoryCommand_WLwWLcBl(strings.Split(message[1], " "))
	case "/revive":
		if len(message) == 1 && up.pl.dead {
			up.Lock()
			up.pl.dead = false
			up.pl.hitPoints = 0.3
			up.updatedStats = true
			up.pl.coord = up.pl.reviveSP
			up.Unlock()
		}
	case "/level":
		if (up.pl.adminLevel >= 8 || *allowTestUser) && len(message) == 2 {
			lvl, err := strconv.ParseUint(message[1], 10, 0)
			if err != nil {
				up.Printf_Bl("%s", err)
			} else {
				up.pl.level = uint32(lvl)
				up.updatedStats = true
			}
		}
	case "/timers":
		if up.pl.adminLevel >= 2 || *allowTestUser {
			timerstats.Report(up)
		}
	case "/panic":
		if up.pl.adminLevel >= 8 || *allowTestUser {
			log.Panic("client_prot.DEBUG command 'panic'")
		}
	case "/status":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		up.Printf_Bl("!!Status")
		up.Printf_Bl("!Chunks loaded: %d, super chunks %d", worldCacheNumChunks, superChunkManager.Size())
		up.Printf_Bl("!Num players:%v, monsters %v, near monsters %d", numPlayers, len(monsterData.m), CountNearMonsters_RLq(up.GetPreviousPos()))
		up.Printf_Bl("!Mem in use %vMB, total alloc %vMB, num malloc %vM, num free %vM",
			m.Alloc/1e6, m.TotalAlloc/1e6, m.Mallocs/1e6, m.Frees/1e6)
		up.Printf_Bl("!Worst message write %.6f s, Worst chunk read %.6f s", float64(WorstWriteTime)/float64(time.Second), float64(DBStats.WorstRead)/float64(time.Second))
		up.Printf_Bl("!Num chunks read: %d, average read time %.6f", DBStats.NumRead, float64(DBStats.TotRead)/float64(DBStats.NumRead)/float64(time.Second))
		up.Printf_Bl("!Created chunks: %d, average time %.6f", DBCreateStats.Num, float64(DBCreateStats.TotTime)/float64(DBCreateStats.Num)/float64(time.Second))
		up.Printf_Bl("!%s", trafficStatistics)
		WorstWriteTime = 0
		DBStats.WorstRead = 0
	case "/players":
		up.ReportPlayers()
	case "/flying":
		up.pl.flying = !up.pl.flying
		up.pl.climbing = false // Always turn off climbing
		up.Printf_Bl("Flying: %v", up.pl.flying)
	case "/inv":
		fallthrough
	case "/inventory":
		if len(message) == 2 && up.pl.adminLevel > 8 {
			maker, ok := objectTable[message[1]]
			if message[1] == "clear" {
				up.pl.inventory.Clear() // There is no update message generated, so client won't know.
				up.pl.WeaponType = 0
				up.pl.ArmorType = 0
				up.pl.HelmetType = 0
				up.pl.WeaponLvl = 0
				up.pl.ArmorLvl = 0
				up.pl.HelmetLvl = 0
			} else if !ok {
				up.Printf_Bl("!Available objects:")
				for key, maker := range objectTable {
					up.Printf_Bl("!%v (%v)", maker(MonsterDifficulty(&up.pl.coord)), key)
				}
			} else {
				AddOneObjectToUser_WLuBl(up, maker(MonsterDifficulty(&up.pl.coord)))
			}
		} else {
			up.pl.inventory.Report(up)
			up.Printf_Bl("!Equip modifiers: armor %.0f%%, helmet %.0f%%, weapon %.0f%%",
				(ArmorLevelDiffMultiplier(up.pl.level, up.pl.ArmorLvl, up.pl.ArmorType)-1)*100,
				(ArmorLevelDiffMultiplier(up.pl.level, up.pl.HelmetLvl, up.pl.HelmetType)-1)*100,
				(WeaponLevelDiffMultiplier(up.pl.level, up.pl.WeaponLvl, up.pl.WeaponType)-1)*100)
		}
	case "/GC":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		up.Printf_Bl("GC next: %v, num: %v, paus total: %v", m.NextGC, m.NumGC, m.PauseTotalNs)
		// runtime.GC()
	case "/shutdown":
		if up.pl.adminLevel >= 8 {
			if *cpuprofile != "" {
				pprof.StopCPUProfile()
			}
			GraceFulShutdown()
			// Will not return
		}
	case "/evalsync":
		for _, str := range evalsync.Eval() {
			up.Printf_Bl("!%s", str)
		}
	case "/resetpos":
		up.pl.coord.X = 0
		up.pl.coord.Y = 0
		up.pl.coord.Z = FLOATING_ISLANDS_LIM - PlayerHeight // As high as possible
		up.pl.flying = false
		up.pl.climbing = false
	case "/prof":
		if up.pl.adminLevel >= 8 || *allowTestUser {
			const fn = "profdata.tmp"
			f, _ := os.OpenFile(fn, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
			pprof.WriteHeapProfile(f)
			f.Close()
			up.Printf_Bl("pprof written to %s\n", fn)
		}
	case "/makelicense":
		if up.pl.adminLevel >= 8 && len(message) == 2 {
			args := strings.Split(message[1], " ") // split the rest of the input string into the arguments needed
			if len(args) != 3 {
				up.Printf_Bl("#FAIL Usage: /makelicense email password avatar")
			} else {
				email := args[0]
				oldlic := license.Load_Bl(email)
				if oldlic != nil {
					up.Printf_Bl("email %v already used (avatar %v)", email, oldlic.Names)
				} else {
					lic := license.Make(email)
					lic.NewPassword(args[1])
					lic.License = license.GenerateKey()
					lic.Names = []string{args[2]} // Array of names, initialized with one entry
					ok := lic.Save_Bl()
					if ok {
						up.Printf_Bl("Saved %v", email)
					} else {
						up.Printf_Bl("#FAIL Failed to save %v", email)
					}
				}
			}
		}
	case "/loadlicense":
		if (up.pl.adminLevel >= 8 || *allowTestUser) && len(message) == 2 {
			email := message[1]
			lic := license.Load_Bl(email)
			up.Printf_Bl("%v", lic)
		}
	case "/say":
		if len(message) < 2 {
			break
		}
		near := playerQuadtree.FindNearObjects_RLq(up.GetPreviousPos(), client_prot.NEAR_OBJECTS)
		n := 0
		for _, o := range near {
			other, ok := o.(*user)
			if !ok {
				continue // Only need to tell players, not monsters etc.
			}
			if other == up {
				continue // Found self
			}
			// Tell 'other' that we moved
			other.Printf("%s says: %s", up.pl.name, message[1])
			n++
		}
		if n == 0 {
			up.Printf_Bl("#FAIL No one is near")
		} else {
			up.Printf_Bl("You say: %s", message[1])
		}
	case "/tell":
		if len(message) < 2 {
			break
		}
		up.TellOthers_RLaBl(message[1])
	case "/friend":
		if len(message) < 2 {
			break
		}
		up.FriendCommand_RLaWLu(message[1])
	case "/score":
		score.Report(up)
	case "/target":
		if len(message) < 2 {
			break
		}
		up.TargetCommand(message[1:])
	}
}

func (up *user) TargetCommand(msg []string) {
	if msg == nil || len(msg) == 0 {
		return
	}
	switch msg[0] {
	case "set":
		up.pl.targetCoor = up.pl.coord
		return
	case "show":
		up.Printf_Bl("Current target: %v", up.pl.targetCoor)
		return
	case "reset":
		up.pl.targetCoor = user_coord{0, 0, 0}
	}
}

func (up *user) TerritoryCommand_WLwWLcBl(msg []string) {
	if msg == nil || len(msg) == 0 {
		return
	}
	switch msg[0] {
	case "show":
		up.Printf_Bl("Territory (%d of %d): %v", len(up.pl.territory), up.pl.maxchunks, up.pl.territory)
		if up.pl.adminLevel > 0 {
			cc := up.pl.coord.GetChunkCoord()
			cp := ChunkFind_WLwWLc(cc)
			up.Printf_Bl("adm: This place: %d", cp.owner)
		}
	case "claim":
		up.TerritoryClaim_WLwWLc(msg[1:])
	case "grant":
		if up.pl.adminLevel < 5 || len(msg) != 2 {
			up.Printf_Bl("#FAIL")
			return
		}
		up.TerritoryGrant(msg[1])
	case "revert":
		if up.pl.adminLevel < 10 {
			up.Printf_Bl("#FAIL")
			return
		}
		cc := up.pl.coord.GetChunkCoord()
		cp := ChunkFind_WLwWLc(cc)
		if cp.owner != OWNER_NONE && cp.owner != OWNER_RESERVED {
			up.Printf_Bl("#FAIL Can't revert when owner is %d", cp.owner)
			return
		}
		// Remove the old chunk and make a new one from scratch
		worldCacheLock.Lock()
		RemoveChunkFromHashTable(cp)
		cp = dBCreateAndSaveChunk(cc)
		AddChunkToHashTable(cp)
		worldCacheLock.Unlock()
		up.CmdReadChunk_WLwWLcBl(cc) // Use exisiting method to send chunk
	default:
		up.Printf_Bl("#FAIL Unknown territory command %v", msg[0])
	}
}

func (up *user) TerritoryGrant(arg string) {
	cc := up.pl.coord.GetChunkCoord()
	cp := ChunkFind_WLwWLc(cc)
	newOwner, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		up.Printf_Bl("%v", err)
		return
	}
	up.Printf_Bl("Changed owner from %d to %d", cp.owner, newOwner)
	cp.owner = uint32(newOwner)
	cp.Write()
}

func (up *user) TerritoryClaim_WLwWLc(arg []string) {
	const usage = "Usage: /territory claim [up/down]"
	if up.pl.adminLevel < 1 && len(up.pl.territory) >= up.pl.maxchunks {
		up.Printf_Bl("#FAIL !You are not allowed more chunks than %d", up.pl.maxchunks)
		return
	}
	if *allowTestUser && NameIsTestPlayer(up.pl.name) {
		up.Printf_Bl("#FAIL !Test players can't claim territory")
		return
	}
	if MonsterDifficulty(&up.pl.coord) > up.pl.level && up.pl.adminLevel == 0 {
		up.Printf_Bl("#FAIL !You are too low level for this area")
		return
	}
	if len(arg) > 1 {
		up.Printf_Bl(usage)
		return
	}
	cc := up.pl.coord.GetChunkCoord()
	if len(arg) > 0 {
		switch arg[0] {
		case "up":
			cc.Z++
		case "down":
			cc.Z--
		case "west":
			cc.X--
		case "east":
			cc.X++
		case "south":
			cc.Y--
		case "north":
			cc.Y++
		default:
			up.Printf_Bl(usage)
			return
		}
	}
	cp := ChunkFind_WLwWLc(cc)
	cp.Lock()
	if cp.owner != OWNER_NONE {
		cp.Unlock()
		up.Printf_Bl("#FAIL !Chunk %v is already allocated to ID %d", cc, cp.owner)
		return
	}

	// Make sure either it is the first chunk, or an adjacent chunk is already allocated, or the request will be denied.
	approved := len(up.pl.territory) == 0 || up.pl.adminLevel > 0
	adjacent := dBGetAdjacentChunks(&cc)
	for _, cp := range adjacent {
		if cp.owner == up.uid {
			approved = true
			break
		}
	}
	if !approved {
		up.Printf_Bl("#FAIL !You must allocate adjacent to another of your chunks")
		return
	}

	// All tests are approved, allocate the chunk
	ChunkFind_WLwWLc(chunkdb.CC{cc.X, cc.Y, cc.Z})
	cp.owner = up.uid
	cp.flag |= CHF_MODIFIED
	cp.Write()
	cp.Unlock()
	up.Printf_Bl("!Congratulations, you now own chunk %v", cc)
	if up.pl.territory == nil {
		up.pl.territory = []chunkdb.CC{cc}
	} else {
		for _, chunk := range up.pl.territory {
			if chunk.X == cc.X && chunk.Y == cc.Y && chunk.Z == cc.Z {
				log.Printf("Chunk %v allocated to user %d (%s), but was already in DB list\n", cc, up.uid, up.pl.name)
				return
			}
		}
		up.pl.territory = append(up.pl.territory, cc)
	}
	chunkdb.SaveAvatar_Bl(up.uid, up.pl.territory)
}

func (up *user) TellOthers_RLaBl(arg string) {
	message := strings.SplitN(arg, " ", 2)
	if len(message) != 2 {
		return
	}

	name := message[0]
	allPlayersSem.RLock()
	other, ok := allPlayerNameMap[strings.ToLower(name)]
	allPlayersSem.RUnlock()

	if !ok {
		up.Printf_Bl("#FAIL No player %v logged in", name)
	} else {
		other.Printf("%v tells you: %s", up.pl.name, message[1])
		up.Printf_Bl("You tell %v: %s", name, message[1])
	}
}

// Handle all commands starting with /friend
func (up *user) FriendCommand_RLaWLu(arg string) {
	if up.uid == OWNER_TEST {
		// Test players can't define friends
		return
	}
	cmd := strings.Split(arg, " ")
	switch cmd[0] {
	case "add":
		if len(cmd) != 2 {
			up.Printf_Bl("#FAIL !Usage: /friend add [name]")
			return
		}
		name := cmd[1]
		if up.pl.name == name {
			up.Printf_Bl("#FAIL !Can't add self")
			return
		}
		notFound, alreadyIn := up.AddToListener_RLaWLu(name)
		if notFound {
			up.Printf_Bl("#FAIL !%v must be logged in to add", name)
		} else if alreadyIn {
			up.Printf_Bl("#FAIL !%v is already on your friends list", name)
		} else {
			up.Printf_Bl("!%v added to your friends list", name)
		}
	case "remove":
		if len(cmd) != 2 {
			up.Printf_Bl("#FAIL !Usage: /friend remove [name]")
			return
		}
		name := cmd[1]
		notFound, notIn := up.RemoveFromListener_RLaWLu(name)
		if notFound {
			up.Printf_Bl("#FAIL !%v must be logged in to remove", name)
		} else if notIn {
			up.Printf_Bl("#FAIL !%v was not on your friends list", name)
		} else {
			up.Printf_Bl("!%v removed from your friends list", name)
		}
	}
}

func GraceFulShutdown() {
	score.Close()
	SaveAllPlayers_RLa() // This will only set the flag to save
	time.Sleep(1e9)      // TODO: not a pretty way. Wait for players to be saved.
	os.Exit(0)
}

func (up *user) ActivatorControl(msg string) {
	cmd := strings.SplitN(string(msg), " ", 2)
	switch cmd[0] {
	case "show":
		cc := up.pl.coord.GetChunkCoord()
		cp := ChunkFind_WLwWLc(cc)
		for _, tr := range cp.blTriggers {
			up.Printf_Bl("!Trigger: %v", tr)
		}
		up.Printf_Bl("!Messages: %v", cp.triggerMsgs)
	case "clear":
		if len(cmd) < 2 {
			up.Printf_Bl("#FAIL !Usage: /activator clear chunkX chunkY chunkZ X Y Z")
			return
		}
		var x, y, z uint8
		var cc chunkdb.CC
		n, err := fmt.Sscan(cmd[1], &cc.X, &cc.Y, &cc.Z, &x, &y, &z)
		// up.Printf_Bl("scan result: %d,%d,%d, err %v, n %d", x, y, z, err, n)
		if err != nil || n != 6 {
			return
		}
		cp := ChunkFind_WLwWLc(cc)
		cp.Lock()
		msgp := cp.FindActivator(x, y, z)
		if msgp != nil {
			*msgp = nil
		} else {
			log.Println("Failed to find text message", x, y, z, cp.coord)
		}
		cp.Write()
		cp.Unlock()
	case "add":
		if len(cmd) < 2 {
			up.Printf_Bl("#FAIL !Usage: /activator add chunkX chunkY chunkZ X Y Z MSG")
			return
		}
		var x, y, z uint8
		var cc chunkdb.CC
		n, err := fmt.Sscan(cmd[1], &cc.X, &cc.Y, &cc.Z, &x, &y, &z)
		// up.Printf_Bl("scan result: %d,%d,%d, err %v, n %d", x, y, z, err, n)
		if err != nil || n != 6 {
			return
		}
		cp := ChunkFind_WLwWLc(cc)
		tmp := strings.SplitN(cmd[1], " ", 7)
		if len(tmp) != 7 {
			up.Printf_Bl("#FAIL !Missing string at end")
			return
		}
		// up.Printf_Bl("!Adding activator to chunk %v at %d,%d,%d to '%s'", cc, x, y, z, tmp[6])
		cp.Lock()
		msgp := cp.FindActivator(x, y, z)
		if msgp != nil {
			*msgp = append(*msgp, tmp[6])
		} else {
			log.Println("Failed to find text message", x, y, z, cp.coord)
		}
		cp.Write()
		cp.Unlock()
	}
}

func (up *user) ReportPlayers() {
	allPlayersSem.RLock()
	for _, p := range allPlayerIdMap {
		switch p.connState {
		case PlayerConnStateLogin:
			up.Printf_Bl("!%v state login", p.pl.name)
		case PlayerConnStatePass:
			up.Printf_Bl("!%v state password", p.pl.name)
		case PlayerConnStateIn:
			up.Printf_Bl("!%v level %d at chunk %v", p.pl.name, p.pl.level, p.pl.coord.GetChunkCoord())
		default:
			up.Printf_Bl("!%v (unknown state)", p.pl.name)
		}
	}
	allPlayersSem.RUnlock()
}
