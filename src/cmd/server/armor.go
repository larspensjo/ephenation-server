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
type Armor struct {
	Type  uint8 // 0 is no armor, higher is better
	Count uint8
	Level uint32
}

func ConvertArmorTypeToID(wpType uint8) (ret string) {
	switch wpType {
	case 0:
		ret = ItemArmorNoneId
	case 1:
		ret = ItemArmor1ID
	case 2:
		ret = ItemArmor2ID
	case 3:
		ret = ItemArmor3ID
	case 4:
		ret = ItemArmor4ID
	}
	return
}

func (arm *Armor) ID() string {
	return ConvertArmorTypeToID(arm.Type)
}

func MakeArmor4(level uint32) inventory.Object {
	return &Armor{Level: level, Type: 4, Count: 1}
}

func MakeArmor3(level uint32) inventory.Object {
	return &Armor{Level: level, Type: 3, Count: 1}
}

func MakeArmor2(level uint32) inventory.Object {
	return &Armor{Level: level, Type: 2, Count: 1}
}

func MakeArmor1(level uint32) inventory.Object {
	return &Armor{Level: level, Type: 1, Count: 1}
}

func (oldItem *Armor) AddTest(o inventory.Object) bool {
	newArmor, ok := o.(*Armor)
	if !ok {
		return false
	}
	if newArmor.Type == oldItem.Type && newArmor.Level == oldItem.Level {
		if oldItem.Count < 255 {
			oldItem.Count++
		}
		return true
	}
	return false
}

func (*Armor) Classification() inventory.TClassification {
	return inventory.TArmor
}

func (arm *Armor) GetCount() uint8 {
	return arm.Count
}

func (item *Armor) DecrementCount() uint8 {
	item.Count--
	return item.Count
}

func (arm *Armor) GetLevel() uint32 {
	return arm.Level
}

func (arm *Armor) String() (ret string) {
	switch arm.Type {
	case 0:
		ret = "no armor"
	case 1:
		ret = "a T-shirt"
	case 2:
		ret = "an average armor"
	case 3:
		ret = "a plate mail"
	case 4:
		ret = "a plate male with magic reinforcement"
	default:
		ret = "unknown armor"
	}
	return
}

// Wield an armor. The argument is always a user pointer. Return a function
// that actually does the job.
func (arm *Armor) Use(p interface{}) func() bool {
	up, ok := p.(*user)
	if !ok || up == nil {
		panic("Not a user pointer")
	}
	f := func() (ret bool) {
		replaced := false
		up.Lock()
		if up.pl.ArmorLvl+uint32(up.pl.ArmorType) < arm.Level+uint32(arm.Type) {
			up.pl.ArmorType = arm.Type
			up.pl.ArmorLvl = arm.Level
			up.pl.inventory.Remove(arm.ID(), arm.Level)
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

func (itm *Armor) Value(playerLevel uint32) float32 {
	return ItemValueAsDrop(playerLevel, itm.Level, itm.Type)
}
