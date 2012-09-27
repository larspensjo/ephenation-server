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
type Weapon struct {
	Type  uint8
	Count uint8
	Level uint32
}

func ConvertWeaponTypeToID(wpType uint8) (ret string) {
	switch wpType {
	case 0:
		ret = ItemWpnHandsID
	case 1:
		ret = ItemWeapon1ID
	case 2:
		ret = ItemWeapon2ID
	case 3:
		ret = ItemWeapon3ID
	case 4:
		ret = ItemWeapon4ID
	}
	return
}

func (wp *Weapon) ID() string {
	return ConvertWeaponTypeToID(wp.Type)
}

func MakeWeapon4(level uint32) inventory.Object {
	return &Weapon{Level: level, Type: 4, Count: 1}
}

func MakeWeapon3(level uint32) inventory.Object {
	return &Weapon{Level: level, Type: 3, Count: 1}
}

func MakeWeapon2(level uint32) inventory.Object {
	return &Weapon{Level: level, Type: 2, Count: 1}
}

func MakeWeapon1(level uint32) inventory.Object {
	return &Weapon{Level: level, Type: 1, Count: 1}
}

func (oldItem *Weapon) AddTest(o inventory.Object) bool {
	newWeapon, ok := o.(*Weapon)
	if !ok {
		return false
	}
	if newWeapon.Type == oldItem.Type && newWeapon.Level == oldItem.Level {
		if oldItem.Count < 255 {
			oldItem.Count++
		}
		return true
	}
	return false
}

func (*Weapon) Classification() inventory.TClassification {
	return inventory.TWeapon
}

func (wp *Weapon) GetCount() uint8 {
	return wp.Count
}

func (item *Weapon) DecrementCount() uint8 {
	item.Count--
	return item.Count
}

func (wp *Weapon) GetLevel() uint32 {
	return wp.Level
}

func (wp *Weapon) String() (ret string) {
	switch wp.Type {
	case 0:
		ret = "bare hands"
	case 1:
		ret = "a sword"
	case 2:
		ret = "a fine sword"
	case 3:
		ret = "a mighty sword"
	case 4:
		ret = "shining blue sword with engraved gems"
	default:
		ret = "unknown weapon"
	}
	return
}

// Wield a weapon. The argument is always a user pointer. Return a function
// that actually does the job.
func (wp *Weapon) Use(p interface{}) func() bool {
	up, ok := p.(*user)
	if !ok || up == nil {
		panic("Not a user pointer")
	}
	f := func() (ret bool) {
		replaced := false
		up.Lock()
		if up.pl.WeaponLvl+uint32(up.pl.WeaponType) < wp.Level+uint32(wp.Type) {
			up.pl.WeaponType = wp.Type
			up.pl.WeaponLvl = wp.Level
			up.pl.inventory.Remove(wp.ID(), wp.Level)
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

func (wp *Weapon) Value(playerLevel uint32) float32 {
	return ItemValueAsDrop(playerLevel, wp.Level, wp.Type)
}
