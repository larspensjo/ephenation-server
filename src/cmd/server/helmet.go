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
	"inventory"
)

// This type implements the inventory.Object interface.
type Helmet struct {
	Type  uint8 // 0 is no armor, higher is better
	Count uint8
	Level uint32
}

func ConvertHelmetTypeToID(wpType uint8) (ret string) {
	switch wpType {
	case 0:
		ret = ItemHelmetNoneId
	case 1:
		ret = ItemHelmet1ID
	case 2:
		ret = ItemHelmet2ID
	case 3:
		ret = ItemHelmet3ID
	case 4:
		ret = ItemHelmet4ID
	}
	return
}

func (itm *Helmet) ID() string {
	return ConvertHelmetTypeToID(itm.Type)
}

func MakeHelmet4(level uint32) inventory.Object {
	return &Helmet{Level: level, Type: 4, Count: 1}
}

func MakeHelmet3(level uint32) inventory.Object {
	return &Helmet{Level: level, Type: 3, Count: 1}
}

func MakeHelmet2(level uint32) inventory.Object {
	return &Helmet{Level: level, Type: 2, Count: 1}
}

func MakeHelmet1(level uint32) inventory.Object {
	return &Helmet{Level: level, Type: 1, Count: 1}
}

func (oldItem *Helmet) AddTest(o inventory.Object) bool {
	newHelmet, ok := o.(*Helmet)
	if !ok {
		return false
	}
	if newHelmet.Type == oldItem.Type && newHelmet.Level == oldItem.Level {
		if oldItem.Count < 255 {
			oldItem.Count++
		}
		return true
	}
	return false
}

func (*Helmet) Classification() inventory.TClassification {
	return inventory.THelmet
}

func (itm *Helmet) GetCount() uint8 {
	return itm.Count
}

func (item *Helmet) DecrementCount() uint8 {
	item.Count--
	return item.Count
}

func (itm *Helmet) GetLevel() uint32 {
	return itm.Level
}

func (itm *Helmet) String() (ret string) {
	switch itm.Type {
	case 0:
		ret = "no helmet"
	case 1:
		ret = "straw hat"
	case 2:
		ret = "helmet"
	case 3:
		ret = "shiny helmet"
	case 4:
		ret = "dragon helmet"
	default:
		ret = "unknown helmet"
	}
	return
}

// Wield an armor. The argument is always a user pointer. Return a function
// that actually does the job.
func (itm *Helmet) Use(p interface{}) func() bool {
	up, ok := p.(*user)
	if !ok || up == nil {
		panic("Not a user pointer")
	}
	f := func() (ret bool) {
		replaced := false
		up.Lock()
		if up.pl.HelmetLvl+uint32(up.pl.HelmetType) < itm.Level+uint32(itm.Type) {
			up.pl.HelmetType = itm.Type
			up.pl.HelmetLvl = itm.Level
			up.pl.Inventory.Remove(itm.ID(), itm.Level)
			replaced = true
		}
		up.Unlock()
		if replaced {
			ReportEquipmentToNear_Bl(up)
			ret = true
		} else {
			up.Printf_Bl("#FAIL Inferior weapon")
		}
		return
	}
	return f
}

func (itm *Helmet) Value(playerLevel uint32) float32 {
	return ItemValueAsDrop(playerLevel, itm.Level, itm.Type)
}
