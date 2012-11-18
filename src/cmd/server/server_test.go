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
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"testing"
)

func TestInventory(t *testing.T) {
	var up user
	up.conn = MakeDummyConn()
	inv := &up.pl.Inventory
	if inv.Len() != 0 {
		t.Error("Not empty", inv)
	}

	inv.Add(MakeHealthPotion(0))
	if inv.Len() != 1 {
		t.Error("Failed to add", inv)
	}

	inv.Add(MakeHealthPotion(0))
	if inv.Len() != 1 {
		t.Error("List should still have length 1", inv)
	}

	// Add another type of potion, which shall not increase the same counter
	inv.Add(MakeManaPotion(0))
	if inv.Len() != 2 {
		t.Error("Shall be two entries", inv)
	}

	// Add mana  potion again
	inv.Add(MakeManaPotion(0))
	if inv.Len() != 2 {
		t.Error("Shall still be two entries", inv)
	}

	if up.pl.HitPoints != 0 || up.pl.Mana != 0 {
		t.Error("Initial condition for hp and mana wrong")
		t.FailNow()
	}

	inv.Use(ItemHealthPotionID, 0, &up)()
	if up.pl.HitPoints == 0 {
		t.Error("Failed to use healing pot", inv)
	}
	if inv.Len() != 2 {
		t.Error("Shall still be two entries", inv)
	}

	inv.Use(ItemHealthPotionID, 0, &up)()
	if inv.Len() != 1 {
		t.Error("Only mana potions", inv)
	}

	inv.Use(ItemManaPotionID, 0, &up)()
	if up.pl.Mana == 0 {
		t.Error("Failed to use mana potion")
	}
	if inv.Len() != 1 {
		t.Error("Only mana potions", inv)
	}

	inv.Use(ItemManaPotionID, 0, &up)()
	if inv.Len() != 0 {
		t.Error("Shall be empty inventory", inv)
	}
}

func BenchmarkFilesystem(b *testing.B) {
	b.StopTimer()
	dirs, err := ioutil.ReadDir("DB")
	if err != nil {
		b.Error(err)
		b.FailNow()
		return
	}
	num := float64(len(dirs))
	log.Println("Number of entries:", num)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		n := int(rand.Float64() * num)
		name := CnfgChunkFolder + "/" + dirs[n].Name()
		f, err := os.Open(name)
		if err != nil {
			b.Error(name, err)
			b.FailNow()
			return
		}
		f.Close()
	}
}
