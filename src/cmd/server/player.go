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
	"encoding/json"
	"ephenationdb"
	"fmt"
	"inventory"
	"keys"
	"log"
	"math"
	"math/rand"
	"score"
	"strconv"
	"strings"
	"time"
)

// All this data is saved with the player. Only fields with upper case prefix are saved.
// All functions in this file are only called from the client process.
// TODO: Horisontal direction angle should be 0 radians to the east, as it is for monsters.
type player struct {
	ZSpeed     float64      // Upward movement speed
	logonTimer time.Time    // Store time to count user online time
	name       string       // The name of the player
	coord      user_coord   // The current player feet coordinates
	adminLevel uint8        // A constant from Admin*, used to control the rights.
	flying     bool         // True if player is moving without any gravity.
	climbing   bool         // True if player is on a ladder. It is similar to the flyng mode.
	dead       bool         // True if the player is dead.
	WeaponType uint8        // 0 is no weapon, higher is better.
	ArmorType  uint8        // 0 is no armor, higher is better.
	HelmetType uint8        // 0 is no helmet, higher is better.
	WeaponLvl  uint32       // The level where the weapon was found.
	ArmorLvl   uint32       // The level where the armor was found.
	HelmetLvl  uint32       // The level where the helmet was found.
	dirHor     float32      // The horistonal direction the player is looking at. 0 radians means looking to the north. This is controlled by the client.
	dirVert    float32      // The vertical direction the player is looking at. 0 radians means horisontal This is controlled by the client.
	level      uint32       // The player level
	exp        float32      // Experience points, from 0-1. At 1.0, the next level is reached
	hitPoints  float32      // Player hit points, 0-1. 1 is full hp, 0 is dead.
	mana       float32      // Player mana, 0-1.
	numKill    uint32       // Total number of monster kills. Just for fun.
	homeSP     user_coord   // Your home spawn, if any.
	reviveSP   user_coord   // This is where you come when you die.
	targetCoor user_coord   // Used for targeting mechanisms
	territory  []chunkdb.CC // The chunks allocated for this player
	Listeners  []uint32     // The list of people listening on this player logging in and out.
	maxchunks  int          // Max number of chunks this player can own.
	blockAdd   uint32       // Blocks added by this player
	blockRem   uint32       // Blocks removed by this player
	timeOnline uint32       // Current time online
	head       uint16       // Head type
	body       uint16       // Body type
	Keys       keys.KeyRing // The list of keys that the player has
	inventory  inventory.Inventory
}

func (this *player) String() string {
	return fmt.Sprintf("[%s %v]", this.name, this.coord)
}

// Return true if ok, and the uid of the player.
// TODO: Improve error handlng
func (pl *player) Load_WLwBlWLc(name string) (uint32, bool) {
	// Connect to database
	db := ephenationdb.New()
	if db == nil {
		return 0, false
	}
	defer ephenationdb.Release(db)

	// Build a query for the avatar name sent as an argument
	// TODO: Assert that the avatar name is unique and on this server for the current user?
	query := "SELECT jsonstring,id,PositionX,PositionY,PositionZ,isFlying,isClimbing,isDead,DirHor,DirVert,AdminLevel,Level,Experience,HitPoints,Mana,Kills,HomeX,HomeY,HomeZ,ReviveX,ReviveY,ReviveZ,maxchunks,BlocksAdded,BlocksRemoved,TimeOnline,HeadType,BodyType,inventory,TScoreTotal,TScoreBalance,TScoreTime,TargetX,TargetY,TargetZ FROM avatars WHERE name='" + name + "'"
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Println(err)
		return 0, false
	}

	// Execute statement
	err = stmt.Execute()
	if err != nil {
		log.Println(err)
		return 0, false
	}

	// Some helper variables
	var packedline string
	var uid uint32
	var packedInv []byte
	var terrScore, terrScoreBalance float64
	var terrScoreTimestamp uint32
	// Booleans doesn't work
	var flying, climbing, dead int
	stmt.BindResult(&packedline, &uid, &pl.coord.X, &pl.coord.Y, &pl.coord.Z, &flying, &climbing, &dead, &pl.dirHor, &pl.dirVert, &pl.adminLevel, &pl.level,
		&pl.exp, &pl.hitPoints, &pl.mana, &pl.numKill, &pl.homeSP.X, &pl.homeSP.Y, &pl.homeSP.Z, &pl.reviveSP.X, &pl.reviveSP.Y, &pl.reviveSP.Z, &pl.maxchunks,
		&pl.blockAdd, &pl.blockRem, &pl.timeOnline, &pl.head, &pl.body, &packedInv, &terrScore, &terrScoreBalance, &terrScoreTimestamp,
		&pl.targetCoor.X, &pl.targetCoor.Y, &pl.targetCoor.Z)

	for {
		eof, err := stmt.Fetch()
		if err != nil {
			log.Println(err)
			return 0, false
		}
		if eof {
			break
		}
	}

	// log.Println(pl.targetCoor)

	// Some post processing
	pl.name = name
	if flying == 1 {
		pl.flying = true
	}
	if climbing == 1 {
		pl.climbing = true
	}
	if dead == 1 {
		pl.dead = true
	}

	if pl.maxchunks == -1 {
		// This parameter was not initialized.
		pl.maxchunks = CnfgMaxOwnChunk
	}

	pl.logonTimer = time.Now()

	if err = json.Unmarshal([]uint8(packedline), pl); err != nil {
		log.Printf("Unmarshal player %s: %v (%v)\n", name, err, packedline)
		// TODO: This covers errors when updating the jsonstring, should be handled in a more approperiate way
		//return 0, false
	}

	// If there was data in the inventory "blob", unpack it.
	if len(packedInv) > 0 {
		err = pl.inventory.Unpack([]byte(packedInv))
		if err != nil {
			log.Println("Failed to unpack", err, packedInv)
		}
		// Save what can be saved, and remove unknown objects.
		pl.inventory.CleanUp()
	}
	if *verboseFlag > 1 {
		log.Println("Inventory unpacked", pl.inventory)
	}

	//fmt.Printf("Coord: (%v,%v,%v)\n", pl.coord.X, pl.coord.Y, pl.coord.Z )

	if pl.reviveSP.X == 0 && pl.reviveSP.Y == 0 && pl.reviveSP.Z == 0 {
		// Check if there is any spawn point defined.
		pl.reviveSP = pl.coord
		pl.homeSP = pl.coord
	}

	// Load the allocated territories. This is loaded every time a player logs in, but not at logout or player save.
	// It will however be updated immediately when the player changes his allocation.
	terr, ok := chunkdb.ReadAvatar_Bl(uint32(uid))
	if !ok {
		return 0, false
	}
	pl.territory = terr
	score.Initialize(uid, terrScore, terrScoreBalance, terrScoreTimestamp, name, len(terr))
	// fmt.Printf("User: %#v\n", pl)
	return uint32(uid), true
}

func Q(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TODO: Improve error handling
// Return true if the save succeeded.
func (pl *player) Save_Bl() bool {
	// Connect to database
	start := time.Now()
	db := ephenationdb.New()
	if db == nil {
		return false
	}

	// Do some preprocessing
	b, err := json.Marshal(pl)
	if err != nil {
		log.Printf("Marshal %s returned %v\n", pl.name, err)
	}

	inventory, err := pl.inventory.Serialize()

	// Update last seen online
	now := time.Now()
	nowstring := fmt.Sprintf("%4v-%02v-%02v", now.Year(), int(now.Month()), now.Day())

	// Update total time online, in seconds
	TempTimer := time.Now()
	pl.timeOnline += uint32(TempTimer.Sub(pl.logonTimer) / time.Second)
	//fmt.Printf("User has been online %v seconds\n", TempTimer - pl.logonTimer)
	pl.logonTimer = TempTimer // Use the stored value to avoid mismatch

	// Write data on alternative format
	// This section makes the following assumptions:
	// - Avatar name cannot be changed from the server
	// - Avatar looks (head/body) cannot be changed from the server TODO: THIS WILL EVENTUALLY NOT BE TRUE!
	// Booleans doesn't work, transform them to numbers
	query := "UPDATE avatars SET jsonstring=?,PositionX=?,PositionY=?,PositionZ=?,isFlying=?,isClimbing=?,isDead=?,DirHor=?,DirVert=?,AdminLevel=?" +
		",Level=?,Experience=?,HitPoints=?,Mana=?,Kills=?,HomeX=?,HomeY=?,HomeZ=?,ReviveX=?,ReviveY=?,ReviveZ=?" +
		",maxchunks=?,BlocksAdded=?,BlocksRemoved=?,TimeOnline=?,HeadType=?,BodyType=?,lastseen=?,inventory=?,TargetX=?,TargetY=?,TargetZ=?" +
		" WHERE name='" + pl.name + "'"

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Println(err)
		return false
	}

	stmt.BindParams(b, pl.coord.X, pl.coord.Y, pl.coord.Z, Q(pl.flying), Q(pl.climbing), Q(pl.dead), pl.dirHor, pl.dirVert, pl.adminLevel, pl.level,
		pl.exp, pl.hitPoints, pl.mana, pl.numKill, pl.homeSP.X, pl.homeSP.Y, pl.homeSP.Z, pl.reviveSP.X, pl.reviveSP.Y, pl.reviveSP.Z, pl.maxchunks,
		pl.blockAdd, pl.blockRem, pl.timeOnline, pl.head, pl.body, nowstring, inventory, pl.targetCoor.X, pl.targetCoor.Y, pl.targetCoor.Z)

	err = stmt.Execute()
	if err != nil {
		log.Println(err)
		return false
	}

	ephenationdb.Release(db)
	if *verboseFlag > 1 {
		log.Printf("up.Save_Bl saved %v\n", pl.name)
	}
	elapsed := time.Now().Sub(start)
	if *verboseFlag > 0 {
		log.Printf("up.Save_Bl elapsed %d ms\n", elapsed/1e6)
	}
	return true
}

// TODO: It looks like this function is only used by test players!
// Return the uid of the player.
func (pl *player) New_WLwWLc(name string) (uid uint32) {
	// All players get a new struct, so there is no need to initialize 0 items.
	pl.name = name
	pl.hitPoints = 1
	pl.mana = 1
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
		uid = OWNER_TEST - uint32(num)
		// fmt.Printf("Test prefix, substr: %v, x,y : (%d,%d)\n", name[len(TestPlayerNamePrefix):], x, y)
	} else {
		log.Println("A player that was not a test player")
	}

	coord := user_coord{x, y, FLOATING_ISLANDS_LIM - 1} // Try this

	for ; coord.Z >= 0; coord.Z -= 1 {
		if !blockIsPermeable[DBGetBlockCached_WLwWLc(coord)] {
			break
		}
	}
	coord.Z += 1

	pl.coord = coord
	pl.reviveSP = coord
	// log.Printf("player.New %s at coord %v\n", pl.name, pl.coord)

	return
}

func NameIsTestPlayer(name string) bool {
	return strings.HasPrefix(name, CnfgTestPlayerNamePrefix)
}
