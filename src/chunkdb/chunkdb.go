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

package chunkdb

//
// The purpose of this package is to manage access to the chunk list table ("chunkdata").
// This table contains information about what users own what chunks.
//

import (
	"ephenationdb"
	"flag"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"os"
)

// A chunk coordinate, an address of any chunk in the world. This will limit the size of the world
// to 0x100000000 * 32 = 1,37E11 blocks
type CC struct {
	// Values are scaled by CHUNK_SIZE to get block coordinates
	X, Y, Z int32 // Relative world center
}

type callbackFunc func(data interface{}, avatarID uint32)

type request struct {
	chunk CC
	data  interface{}
	cb    callbackFunc
}

var (
	disableSave = flag.Bool("chunkdb.disablesave", false, "Disable saving any data. Used for testing.")
	ch          = make(chan request, 100)
	db          *mgo.Database
)

// Take a chunk address, a chunk pointer and a callback function.
// Retrieve the owner (avatar ID) of the chunk, and call the callback function
// TODO: This isn't used any longer
func Update(chunk CC, data interface{}, cb callbackFunc) {
	// The following statement will send to 'ch' if possible, otherwise return and forget about it.
	select {
	case ch <- request{chunk, data, cb}:
	default:
	}
}

// This is the main polling function, meant to be executed for ever as a go routine.
func Poll_Bl() {
	if *disableSave {
		log.Println("Saving to 'chunkdata' is disabled")
	}
	db = ephenationdb.New()
	if db == nil {
		log.Println("chunkdb.Poll_Bl requires access to SQL DB")
		return
	}
	for {
		req, ok := <-ch
		if !ok {
			log.Printf("Failed to read from channel\n")
			os.Exit(1)
		}
		avatarId := readChunk_Bl(req.chunk)
		req.cb(req.data, avatarId)
	}
}

// Given only the LSB of the chunk coordinate, compute the full coordinate, relative to
// a given coordinate. A requirement is that the distance from the relative chunk is small.
func (this CC) UpdateLSB(x, y, z uint8) (ret CC) {
	ret = this
	ret.Z = (ret.Z & ^0xFF) | int32(z) // Replace LSB
	ret.X = (ret.X & ^0xFF) | int32(x) // Replace LSB
	ret.Y = (ret.Y & ^0xFF) | int32(y) // Replace LSB
	// Check for wrap around, which can happen near byte boundary
	if this.X-ret.X > 127 {
		ret.X += 0x100
	}
	if this.Y-ret.Y > 127 {
		ret.Y += 0x100
	}
	if this.Z-ret.Z > 127 {
		ret.Z += 0x100
	}
	if ret.X-this.X > 127 {
		ret.X -= 0x100
	}
	if ret.Y-this.Y > 127 {
		ret.Y -= 0x100
	}
	if ret.Z-this.Z > 127 {
		ret.Z -= 0x100
	}
	return
}

// Find the avatar ID for a chunk. Return 0 when none found.
func readChunk_Bl(chunk CC) uint32 {
	var Id struct {
		AvatarID uint32
	}
	err := db.C("chunkdata").Find(bson.M{"x": chunk.X, "y": chunk.Y, "z": chunk.Z}).One(&Id)
	if err != nil {
		log.Println(err)
		os.Exit(1)
		return 0
	}
	return Id.AvatarID
}

// Find the list of all chunks allocated for an avatar
// Return false in case of failure
func ReadAvatar_Bl(avatarID uint32) ([]CC, bool) {
	db = ephenationdb.New()
	var ret []CC
	err := db.C("chunkdata").Find(bson.M{"avatarID": avatarID}).All(&ret)
	if err != nil {
		log.Println(err)
		return nil, false
	}
	return ret, true
}

// Save the allocated chunks for the specified avatar
func SaveAvatar_Bl(avatar uint32, chunks []CC) bool {
	if *disableSave {
		// Ignore the save, pretend everything is fine
		return true
	}
	c := ephenationdb.New().C("chunkdata")

	if len(chunks) == 0 {
		return true
	}
	for _, ch := range chunks {
		_, err := c.Upsert(bson.M{"x": ch.X, "y": ch.Y, "z": ch.Z}, bson.M{"x": ch.X, "y": ch.Y, "z": ch.Z, "avatarID": avatar})
		if err != nil {
			log.Println(err)
			return false
		}
	}
	return true
}

// Compare two chunk coordinates for equality
func (chunk CC) Equal(chunk2 CC) bool {
	return chunk.X == chunk2.X && chunk.Y == chunk2.Y && chunk.Z == chunk2.Z
}
