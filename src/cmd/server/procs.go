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
// This is a scheduler for a number of processes.
//
// Currently, the scheduling principle will slow down the frequency
// in case of heavy load, instead of always schedule relative to
// the last invocation.
//

import (
	"github.com/robfig/goconfig/config"
	"log"
	"os"
	"os/signal"
	"time"
	"timerstats"
)

func CatchSig() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, os.Kill)
	for {
		sig := <-ch
		log.Printf("Got signal %v %#v\n", sig, sig)
		if sig == os.Interrupt || sig == os.Kill { // 2: kill -HUP, Ctrl-C
			GraceFulShutdown()
			// Will not return
		} else { // kill with no arguments
			panic("Got signal\n")
		}
	}
}

func ProcPurgeOldChunks_WLw() {
	for i := 0; ; i++ {
		if i == WORLD_CACHE_SIZE {
			i = 0
		}
		worldCacheLock.Lock()
		for pc := world_cache[i]; pc != nil; {
			next := pc.next // Save the next pointer now, as the current 'pc' may get released
			if !pc.touched {
				// No one used this chunk since last checked, so it is released
				RemoveChunkFromHashTable(pc)
				// fmt.Printf("ProcPurgeOldChunks: free %v\n", pc.Coord)
			} else {
				// The chunk was touched. Give it another chance.
				pc.touched = false
				// fmt.Printf("ProcPurgeOldChunks: keep %v\n", pc.Coord)
			}
			pc = next
		}
		worldCacheLock.Unlock()
		time.Sleep(1e8)
	}
}

func ProcUpdateMonsterState() {
	var elapsed time.Duration
	timerstats.Add("ProcUpdateMonsterState", MonstersUpdateDirPeriod, &elapsed)
	lastTime := time.Now()
	for {
		start := time.Now()
		time.Sleep(MonstersUpdateDirPeriod)
		if start.Sub(lastTime) > 4e9 {
			lastTime = start
		}
		UpdateAllMonstersState()
		elapsed = time.Now().Sub(start)
	}
}

func ProcUpdateMonsterPos_RLmWLwWLqBlWLuWLc() {
	var elapsed time.Duration
	timerstats.Add("ProcUpdateMonsterPos", ObjectsUpdatePeriod, &elapsed)
	lastTime := time.Now()
	for {
		start := time.Now()
		time.Sleep(ObjectsUpdatePeriod) // TODO: Monsters could move at a lower frequency than players
		fullReport := false
		if start.Sub(lastTime) > 4e9 {
			// Request a full report, but not too often as it generates much traffic
			fullReport = true
			lastTime = start
		}
		UpdateAllMonsterPos_RLmWLwWLqWLuWLc(fullReport)
		elapsed = time.Now().Sub(start)
	}
}

func ProcUpdateMonstersTarget_RLmRLqBl() {
	var elapsed time.Duration
	timerstats.Add("ProcUpdateMonstersTarget", MonstersUpdateTargetPeriod, &elapsed)
	for {
		start := time.Now()
		time.Sleep(MonstersUpdateTargetPeriod)
		UpdateAllMonstersTarget_RLmRLq()
		elapsed = time.Now().Sub(start)
	}
}

func ProcPurgeMonsters_WLmWLqBl() {
	var elapsed time.Duration
	timerstats.Add("ProcPurgeMonsters", PURGE_MONSTER_PERIOD, &elapsed)
	for {
		start := time.Now()
		time.Sleep(PURGE_MONSTER_PERIOD)
		CmdPurgeMonsters_WLmWLq()
		elapsed = time.Now().Sub(start)
	}
}

func ProcSpawnMonsters_WLwWLuWLqWLmBlWLc() {
	var elapsed time.Duration
	timerstats.Add("ProcSpawnMonsters", SPAWN_MONSTER_PERIOD, &elapsed)
	for {
		start := time.Now()
		time.Sleep(SPAWN_MONSTER_PERIOD)
		CmdSpawnMonsters_WLwWLuWLqWLmWLc()
		elapsed = time.Now().Sub(start)
	}
}

// Control monsters attacking players
func ProcMonsterMelee_RLmBl() {
	var elapsed time.Duration
	timerstats.Add("ProcMonsterMeelee", CnfgAttackPeriod, &elapsed)
	for {
		start := time.Now()
		time.Sleep(CnfgAttackPeriod)
		CmdMonsterMelee_RLm()
		elapsed = time.Now().Sub(start)
	}
}

var ClientCurrentMajorVersion, ClientCurrentMinorVersion int

// Autosave of players
func ProcAutosave_RLu() {
	var elapsed time.Duration
	timerstats.Add("ProcAutosave", CnfgAutosavePeriod, &elapsed)
	for {
		start := time.Now()
		oldMajor, oldMinor := ClientCurrentMajorVersion, ClientCurrentMinorVersion
		ClientCurrentMajorVersion, ClientCurrentMinorVersion = LoadClientVersionInformation()
		if (oldMajor != ClientCurrentMajorVersion || oldMinor != ClientCurrentMinorVersion) && *verboseFlag > 0 {
			log.Printf("Current client version %d.%d\n", ClientCurrentMajorVersion, ClientCurrentMinorVersion)
		}
		time.Sleep(CnfgAutosavePeriod)
		SaveAllPlayers_RLa()
		elapsed = time.Now().Sub(start)
	}
}

// Extract the version of the current client from the config file. This is done
// repeatedly, which means the definition can be changed "live", while the server is running.
func LoadClientVersionInformation() (int, int) {
	cnfg, err := config.ReadDefault(*configFileName)
	if err == nil && cnfg.HasSection("client") {
		major, err := cnfg.Int("client", "major")
		if err != nil {
			log.Println(*configFileName, "major:", err)
			return 0, 0
		}
		minor, err := cnfg.Int("client", "minor")
		if err != nil {
			log.Println(*configFileName, "minor:", err)
			return 0, 0
		}
		return major, minor
	}
	return 0, 0
}
