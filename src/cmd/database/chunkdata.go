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
	"labix.org/v2/mgo"
	"log"
)

func chunkdata(c *mgo.Collection) {
	var err error
	// Enable a compund key for the x, y and z coordinate.
	index := mgo.Index{
		Key:      []string{"x", "y", "z"}, // Set a compound index
		Unique:   true,
		DropDups: true,
	}
	err = c.EnsureIndex(index)
	if err != nil {
		log.Println("chunkdb EnsureIndex compound", err)
	}

	// Define another key for the avatar ID
	err = c.EnsureIndexKey("avatarID")
	if err != nil {
		log.Println("chunkdb EnsureIndex avatarID", err)
	}
}
