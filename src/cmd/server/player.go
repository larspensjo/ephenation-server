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

// Functions in this file are only called from the client process.

// All this data is saved with the player. Only fields with upper case prefix are saved.
// There may be more data saved in the DB than is shown here.
// TODO: Horisontal direction angle should be 0 radians to the east, as it is for monsters.
type player struct {
	ZSpeed      float64      // Upward movement speed
	Coord       user_coord   // The current player feet coordinates
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
	TimeOnline  uint32       // Total time online in seconds
	Head        uint16       // Head type
	Body        uint16       // Body type
	Keys        keys.KeyRing // The list of keys that the player has
	Lastseen    time.Time    // When player weas last seen in the game
	Inventory   PlayerInv
}

func (up *user) String() string {
	return fmt.Sprintf("[%s %v]", up.Name, up.Coord)
}

// Return true if ok
func (up *user) Load_WLwBlWLc(email string) bool {
	// Connect to database
	db := ephenationdb.New()
	err := db.C("avatars").Find(bson.M{"email": email}).One(&up.UserLoad)
	if err != nil {
		log.Println("Avatar for", email, err)
		return false
	}

	// Some post processing

	if up.Maxchunks == 0 {
		// This parameter was not initialized.
		up.Maxchunks = CnfgMaxOwnChunk
	}

	up.logonTimer = time.Now()

	if up.ReviveSP.X == 0 && up.ReviveSP.Y == 0 && up.ReviveSP.Z == 0 {
		// Check if there is any spawn point defined.
		up.ReviveSP = up.Coord
		up.HomeSP = up.Coord
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
func (up *user) Save_Bl() bool {
	start := time.Now()
	up.Lastseen = start                                             // Update last seen online
	up.TimeOnline += uint32(start.Sub(up.logonTimer) / time.Second) // Update total time online, in seconds
	up.logonTimer = start
	db := ephenationdb.New()
	err := db.C("avatars").UpdateId(up.Id, bson.M{"$set": &up.player}) // Only update the fields found in 'pl'.

	if err != nil {
		log.Println("Save", up.Name, err)
		log.Printf("%#v\n", up)
		return false
	}

	if *verboseFlag > 1 {
		log.Printf("up.Save_Bl saved %v\n", up.Name)
	}
	elapsed := time.Now().Sub(start)
	if *verboseFlag > 0 {
		log.Printf("up.Save_Bl elapsed %d ms\n", elapsed/1e6)
	}
	return true
}

// This is mostly used for testing, and not part of the regular functionality.
func (up *user) New_WLwWLc(name string) {
	// All players get a new struct, so there is no need to initialize 0 items.
	up.Name = name
	up.HitPoints = 1
	up.Mana = 1
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
		up.Id = OWNER_TEST - uint32(num)
		// fmt.Printf("Test prefix, substr: %v, x,y : (%d,%d)\n", name[len(TestPlayerNamePrefix):], x, y)
	}

	coord := user_coord{x, y, FLOATING_ISLANDS_LIM - 1} // Try this

	for ; coord.Z >= 0; coord.Z -= 1 {
		if !blockIsPermeable[DBGetBlockCached_WLwWLc(coord)] {
			break
		}
	}
	coord.Z += 1

	up.Coord = coord
	up.ReviveSP = coord
	// log.Printf("player.New %s at coord %v\n", pl.Name, pl.Coord)

	return
}

func NameIsTestPlayer(name string) bool {
	return strings.HasPrefix(name, CnfgTestPlayerNamePrefix)
}
