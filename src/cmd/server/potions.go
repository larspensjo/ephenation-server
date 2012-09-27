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
	"fmt"
	"inventory"
)

const (
	PotionHealthConst = iota
	PotionManaConst   = iota
)

// This type implements the inventory.Object interface.
type Potion struct {
	PotionType uint8 // The potion type
	Count      uint8 // The number of potions of this type
}

// Use a potion. The argument is always a user pointer. Return a function
// that actually does the job.
func (this *Potion) Use(p interface{}) func() bool {
	up, ok := p.(*user)
	if !ok || up == nil {
		panic("Not a user pointer")
	}
	if up.pl.dead {
		return nil
	}
	var f func() bool
	switch this.PotionType {
	case PotionHealthConst:
		f = func() (ret bool) {
			up.Lock()
			if up.Heal(0.3, 0) {
				up.pl.inventory.Remove(ItemHealthPotionID, this.GetLevel())
				ret = true
			}
			up.Unlock()
			return
		}
	case PotionManaConst:
		f = func() (ret bool) {
			up.Lock()
			if up.Mana(0.3) {
				up.pl.inventory.Remove(ItemManaPotionID, this.GetLevel())
				ret = true
			}
			up.Unlock()
			return
		}
	}
	return f
}

func (potion *Potion) Classification() inventory.TClassification {
	return inventory.TPotion
}

func (this *Potion) AddTest(o inventory.Object) bool {
	p, ok := o.(*Potion)
	if !ok {
		return false
	}
	if p.PotionType == this.PotionType {
		if this.Count < 255 {
			this.Count++
		}
		return true
	}
	return false
}

func (this *Potion) String() (ret string) {
	var descr string
	switch this.PotionType {
	case PotionHealthConst:
		descr = "health"
	case PotionManaConst:
		descr = "mana"
	}
	if this.Count == 1 {
		ret = "One " + descr + " potion"
	} else {
		ret = fmt.Sprintf("%d %s potions", this.Count, descr)
	}
	return
}

func (this *Potion) ID() (ret string) {
	switch this.PotionType {
	case PotionHealthConst:
		ret = ItemHealthPotionID
	case PotionManaConst:
		ret = ItemManaPotionID
	}
	return
}

func (this *Potion) GetCount() uint8 {
	return this.Count
}

func (item *Potion) DecrementCount() uint8 {
	item.Count--
	return item.Count
}

func (this *Potion) GetLevel() uint32 {
	return 0
}

func (pot *Potion) Value(playerLevel uint32) float32 {
	return 1.0
}

// Level independent
func MakeManaPotion(level uint32) inventory.Object {
	return &Potion{PotionManaConst, 1}
}

// Level independent
func MakeHealthPotion(level uint32) inventory.Object {
	return &Potion{PotionHealthConst, 1}
}
