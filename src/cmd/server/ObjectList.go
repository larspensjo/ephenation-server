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
	"client_prot"
	"encoding/gob"
	"inventory"
	"log"
)

// These constants are part of the communication protocol. If they are changed, the client must be updated.
const (
	ItemHealthPotionID = "POTH"
	ItemManaPotionID   = "POTM"
	ItemWpnHandsID     = "WEP0"
	ItemWeapon1ID      = "WEP1"
	ItemWeapon2ID      = "WEP2"
	ItemWeapon3ID      = "WEP3"
	ItemWeapon4ID      = "WEP4"
	ItemArmorNoneId    = "ARM0"
	ItemArmor1ID       = "ARM1"
	ItemArmor2ID       = "ARM2"
	ItemArmor3ID       = "ARM3"
	ItemArmor4ID       = "ARM4"
	ItemHelmetNoneId   = "HLM0"
	ItemHelmet1ID      = "HLM1"
	ItemHelmet2ID      = "HLM2"
	ItemHelmet3ID      = "HLM3"
	ItemHelmet4ID      = "HLM4"
	ItemScrollRessID   = "S001" // A resurrection scroll
)

var (
	// A static list of all objects available. Every object is identified by a unique 4 character string.
	objectTable = map[string]func(level uint32) inventory.Object{
		ItemHealthPotionID: MakeHealthPotion,
		ItemManaPotionID:   MakeManaPotion,
		ItemWeapon1ID:      MakeWeapon1,
		ItemWeapon2ID:      MakeWeapon2,
		ItemWeapon3ID:      MakeWeapon3,
		ItemWeapon4ID:      MakeWeapon4,
		ItemArmor1ID:       MakeArmor1,
		ItemArmor2ID:       MakeArmor2,
		ItemArmor3ID:       MakeArmor3,
		ItemArmor4ID:       MakeArmor4,
		ItemHelmet1ID:      MakeHelmet1,
		ItemHelmet2ID:      MakeHelmet2,
		ItemHelmet3ID:      MakeHelmet3,
		ItemHelmet4ID:      MakeHelmet4,
		ItemScrollRessID:   MakeResurrectionPointScroll,
	}
)

func init() {
	// To make it possible to save the inventory, all object types have to be registered with gob.
	// Use strings of one character to save bandwidth.
	// If the letter for a type is changed, it will be lost for all inventories using the type.
	gob.RegisterName("P", &Potion{})
	gob.RegisterName("W", &Weapon{})
	gob.RegisterName("A", &Armor{})
	gob.RegisterName("H", &Helmet{})
	gob.RegisterName("S", &Scroll{})
}

// Find all players near 'up', including self, and report the equipment of 'up'
func ReportEquipmentToNear_Bl(up *user) {
	nearPlayers := playerQuadtree.FindNearObjects_RLq(up.GetPreviousPos(), client_prot.NEAR_OBJECTS)
	log.Println("Near players", len(nearPlayers))
	for _, o := range nearPlayers {
		if other, ok := o.(*user); ok {
			other.ReportEquipment_Bl(up)
		}
	}
}

// Compute the experience point value of dropping an item. The value is normalized, giving 1 for
// an item of the same level as the player and of the lowest grade.
func ItemValueAsDrop(playerLevel, itemLevel uint32, itemType uint8) float32 {
	diff := 1 + (float32(itemLevel)-float32(playerLevel)+float32(itemType))/2
	if diff < 0 {
		diff = 0
	} else if diff > 3 {
		diff = 3
	}
	return diff
}
