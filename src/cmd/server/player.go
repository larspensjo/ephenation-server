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
	"chunkdb"
	"ephenationdb"
	"fmt"
	"keys"
	"labix.org/v2/mgo/bson"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// All this data is saved with the player. Only fields with upper case prefix are saved.
// All functions in this file are only called from the client process.
// TODO: Horisontal direction angle should be 0 radians to the east, as it is for monsters.
type player struct {
	Id          uint32       `bson:"_id"`
	ZSpeed      float64      // Upward movement speed
	LogonTimer  time.Time    // Store time to count user online time
	Name        string       // The name of the avatar
	Coord       user_coord   // The current player feet coordinates
	AdminLevel  uint8        // A constant from Admin*, used to control the rights.
	Flying      bool         // True if player is moving without any gravity.
	Climbing    bool         // True if player is on a ladder. It is similar to the flyng mode.
	Dead        bool         // True if the player is dead.
	WeaponGrade uint8        // 0 is no weapon, higher is better.
	ArmorGrade  uint8        // 0 is no armor, higher is better.
	HelmetGrade uint8        // 0 is no helmet, higher is better.
	WeaponLvl   uint32       // The level where the weapon was found.
	ArmorLvl    uint32       // The level where the armor was found.
	HelmetLvl   uint32       // The level where the helmet was found.
	DirHor      float32      // The horistonal direction the player is looking at. 0 radians means looking to the north. This is controlled by the client.
	DirVert     float32      // The vertical direction the player is looking at. 0 radians means horisontal This is controlled by the client.
	Level       uint32       // The player level
	Exp         float32      // Experience points, from 0-1. At 1.0, the next level is reached
	HitPoints   float32      // Player hit points, 0-1. 1 is full hp, 0 is dead.
	Mana        float32      // Player mana, 0-1.
	NumKill     uint32       // Total number of monster kills. Just for fun.
	HomeSP      user_coord   // Your home spawn, if any.
	ReviveSP    user_coord   // This is where you come when you die.
	TargetCoor  user_coord   // Used for targeting mechanisms
	Territory   []chunkdb.CC // The chunks allocated for this player.
	Listeners   []uint32     // The list of people listening on this player logging in and out.
	Maxchunks   int          // Max number of chunks this player can own.
	BlockAdd    uint32       // Blocks added by this player
	BlockRem    uint32       // Blocks removed by this player
	TimeOnline  uint32       // Current time online
	Head        uint16       // Head type
	Body        uint16       // Body type
	Keys        keys.KeyRing // The list of keys that the player has
	Lastseen    string       // String format date
	Email       string       // The owner, which is an email
	License     string
	Password    string
	Inventory   PlayerInv
}

func (this *player) String() string {
	return fmt.Sprintf("[%s %v]", this.Name, this.Coord)
}

// Return true if ok
func (pl *player) Load_WLwBlWLc(email string) bool {
	// Connect to database
	db := ephenationdb.New()
	err := db.C("avatars").Find(bson.M{"email": email}).One(pl)
	if err != nil {
		log.Println("Avatar for", email, err)
		return false
	}

	// Some post processing

	if pl.Maxchunks == 0 {
		// This parameter was not initialized.
		pl.Maxchunks = CnfgMaxOwnChunk
	}

	pl.LogonTimer = time.Now()

	if pl.ReviveSP.X == 0 && pl.ReviveSP.Y == 0 && pl.ReviveSP.Z == 0 {
		// Check if there is any spawn point defined.
		pl.ReviveSP = pl.Coord
		pl.HomeSP = pl.Coord
	}

	// fmt.Printf("User: %#v\n", pl)
	return true
}

func Q(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Return true if the save succeeded.
func (pl *player) Save_Bl() bool {
	// Connect to database
	start := time.Now()
	// Update last seen online
	pl.Lastseen = fmt.Sprintf("%4v-%02v-%02v", start.Year(), int(start.Month()), start.Day())
	// Update total time online, in seconds
	pl.LogonTimer = start // Use the stored value to avoid mismatch
	pl.TimeOnline += uint32(start.Sub(pl.LogonTimer) / time.Second)
	db := ephenationdb.New()
	err := db.C("avatars").UpdateId(pl.Id, pl)

	if err != nil {
		log.Println("Save", pl.Name, err)
		return false
	}

	if *verboseFlag > 1 {
		log.Printf("up.Save_Bl saved %v\n", pl.Name)
	}
	elapsed := time.Now().Sub(start)
	if *verboseFlag > 0 {
		log.Printf("up.Save_Bl elapsed %d ms\n", elapsed/1e6)
	}
	return true
}

// This is mostly used for testing, and not part of the regular functionality.
func (pl *player) New_WLwWLc(name string) {
	// All players get a new struct, so there is no need to initialize 0 items.
	pl.Name = name
	pl.HitPoints = 1
	pl.Mana = 1
	// Find ground for the player. Start from top of world and go down one block at a time
	// TODO: the dependencies on chunks must not be done in this process.
	// Players with the names "test" and a number are reserved names for testing. The number is
	// used to dsitribute the player.
	var x, y float64 = 0, 0

	if NameIsTestPlayer(name) {
		// Special handling if it is a test player. These are not saved, and the starting placed is randomized
		// but with a even distribution.
		num, _ := strconv.ParseFloat(name[len(CnfgTestPlayerNamePrefix):], 64) // ignore the error, not critical
		radie := math.Sqrt(num/CnfgTestPlayersPerChunk) * CHUNK_SIZE
		a, b := math.Sincos(rand.Float64() * math.Pi * 2)
		x = b * radie
		y = a * radie
		pl.Id = OWNER_TEST - uint32(num)
		// fmt.Printf("Test prefix, substr: %v, x,y : (%d,%d)\n", name[len(TestPlayerNamePrefix):], x, y)
	}

	coord := user_coord{x, y, FLOATING_ISLANDS_LIM - 1} // Try this

	for ; coord.Z >= 0; coord.Z -= 1 {
		if !blockIsPermeable[DBGetBlockCached_WLwWLc(coord)] {
			break
		}
	}
	coord.Z += 1

	pl.Coord = coord
	pl.ReviveSP = coord
	// log.Printf("player.New %s at coord %v\n", pl.Name, pl.Coord)

	return
}

func NameIsTestPlayer(name string) bool {
	return strings.HasPrefix(name, CnfgTestPlayerNamePrefix)
}
