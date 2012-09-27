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

package inventory

// This package manaages the inventory of a player.

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
)

type TClassification uint8

// What kind of obejct it is.
// Never change these constants when there are players with saved inventory that would no longer match
const (
	TPotion = TClassification(iota)
	TWeapon = TClassification(iota)
	TArmor  = TClassification(iota)
	THelmet = TClassification(iota)
	TScroll = TClassification(iota)
)

// The type of one inventory item
type Object interface {
	// Use this object. Return value is the function that does the actual job, if any.
	// This function, in turn, returns a boolean if the object was consumed.
	Use(arg interface{}) func() bool

	// A function that returns what type of object it is
	Classification() TClassification

	// When adding objects, it may be that they already are in the inventory or that
	// they can be added by incrementing a counter. The Add() function will be used to
	// test this for each object. If it returns true, then the object shall not be added
	// as a new inventory item.
	AddTest(o Object) bool

	// A function that describes the object
	String() string

	// A function that returns a unique 4 character ID
	ID() string

	// Get the count for this object
	GetCount() uint8

	// Decrement the count of this object, and return the new number
	DecrementCount() uint8

	// Get the level of the object
	GetLevel() (level uint32)

	// Return a value, given the current player level.
	// The value is normalized, which means an item of lowest grade, adapted for the current player level, with have value 1.0
	Value(playerLevel uint32) float32
}

type Inventory struct {
	Objects []Object // The slize of objects
}

// Remove inventory item 'index' from the list
func (inv *Inventory) remove(index int) {
	if index > len(inv.Objects) {
		log.Panicln("Illegal index", index, "for", inv)
	}
	last := len(inv.Objects) - 1
	inv.Objects[index] = inv.Objects[last]
	inv.Objects = inv.Objects[0:last] // Reslice it, to drop last element
	return
}

func (inv *Inventory) Remove(code string, lvl uint32) {
	for i, obj := range inv.Objects {
		if obj.ID() == code && obj.GetLevel() == lvl && obj.DecrementCount() == 0 {
			inv.remove(i)
			break
		}
	}
}

// Add a new object to the inventory.
func (inv *Inventory) Add(newObject Object) {
	for _, old := range inv.Objects {
		if old.AddTest(newObject) {
			// The object was handled automatically in the current inventory
			return
		}
	}
	inv.Objects = append(inv.Objects, newObject)
}

func (this *Inventory) String() string {
	return fmt.Sprintf("%v", this.Objects)
}

func (this *Inventory) Len() int {
	return len(this.Objects)
}

// Used for debugging
func (this *Inventory) Report(wr io.Writer) {
	for _, o := range this.Objects {
		fmt.Fprintf(wr, "!%s:%v", o.ID(), o)
	}
	return
}

// Encode an inventory into a byte array. It is assumed that any special types have been registered in gob already.
func (this *Inventory) Serialize() ([]byte, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(this)
	return buffer.Bytes(), err
}

// The opposite to Serialize.
func (this *Inventory) Unpack(data []byte) error {
	buffer := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buffer)
	err := decoder.Decode(this)
	return err
}

// Try to save what can be saved. Unknown objects are stored as nil pointers.
func (this *Inventory) CleanUp() {
again:
	for i := range this.Objects {
		// TODO: There shouldn't be any items with count 0, but there was a bug that produced such entries.
		if this.Objects[i] == nil || this.Objects[i].GetCount() == 0 {
			this.remove(i)
			goto again
		}
	}
}

// Use an object, identified by 'id' and 'lvl'.
// TODO: Making lvl 0 match all objects was only to support an old implementation when
// the level was not available.
func (this *Inventory) Use(id string, lvl uint32, arg interface{}) func() bool {
	var f func() bool
	for _, o := range this.Objects {
		if o.ID() == id && (lvl == 0 || o.GetLevel() == lvl) {
			f = o.Use(arg)
			break
		}
	}
	return f
}

// Get the value of an object, identified by 'id' and 'lvl'.
// Return a negative value when not found
func (inv *Inventory) Value(id string, lvl uint32, playerLevel uint32) (value float32) {
	value = -1
	for _, o := range inv.Objects {
		if o.ID() == id && o.GetLevel() == lvl {
			value = o.Value(playerLevel)
			break
		}
	}
	return
}

func (this *Inventory) Get(index int) Object {
	return this.Objects[index]
}

func (this *Inventory) Clear() {
	this.Objects = nil
}
