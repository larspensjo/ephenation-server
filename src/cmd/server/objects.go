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

//
// Define objects and inventory
//

package main

import (
	"fmt"
	"io"
	"log"
)

type ObjectCode string

type Object struct {
	Type  ObjectCode
	Level uint32
	Count uint32
}

type PlayerInv []Object // The slize of objects

const (
	ItemHealthPotionID ObjectCode = "POTH"
	ItemManaPotionID              = "POTM"
	ItemWpnHandsID                = "WEP0"
	ItemWeapon1ID                 = "WEP1"
	ItemWeapon2ID                 = "WEP2"
	ItemWeapon3ID                 = "WEP3"
	ItemWeapon4ID                 = "WEP4"
	ItemArmorNoneId               = "ARM0"
	ItemArmor1ID                  = "ARM1"
	ItemArmor2ID                  = "ARM2"
	ItemArmor3ID                  = "ARM3"
	ItemArmor4ID                  = "ARM4"
	ItemHelmetNoneId              = "HLM0"
	ItemHelmet1ID                 = "HLM1"
	ItemHelmet2ID                 = "HLM2"
	ItemHelmet3ID                 = "HLM3"
	ItemHelmet4ID                 = "HLM4"
	ItemScrollRessID              = "S001" // A resurrection scroll
)

var (
	// A static list of all objects available. Every object is identified by a unique 4 character string.
	objectUseTable = map[ObjectCode]func(up *user, t ObjectCode, level uint32) (bool, bool){
		ItemHealthPotionID: UsePotion_Wlu,
		"POTM":             UsePotion_Wlu,
		"WEP0":             UseWeapon_Wlu,
		"WEP1":             UseWeapon_Wlu,
		"WEP2":             UseWeapon_Wlu,
		"WEP3":             UseWeapon_Wlu,
		"WEP4":             UseWeapon_Wlu,
		"ARM0":             UseArmor_Wlu,
		"ARM1":             UseArmor_Wlu,
		"ARM2":             UseArmor_Wlu,
		"ARM3":             UseArmor_Wlu,
		"ARM4":             UseArmor_Wlu,
		"HLM0":             UseHelmet_Wlu,
		"HLM1":             UseHelmet_Wlu,
		"HLM2":             UseHelmet_Wlu,
		"HLM3":             UseHelmet_Wlu,
		"HLM4":             UseHelmet_Wlu,
		"S001":             UseScroll_Wlu, // A resurrection scroll
	}
)

// Get the index of an item
func (inv *PlayerInv) Find(t ObjectCode, level uint32) int {
	for i, obj := range *inv {
		if obj.Type == t && obj.Level == level {
			return i
		}
	}
	log.Println("Object", t, "level", level, "not found in", inv)
	return -1
}

func (inv *PlayerInv) Clear() {
	*inv = nil
}

func (inv *PlayerInv) AddOneObject(t ObjectCode, level uint32) {
	for i, old := range *inv {
		if old.Type == t && old.Level == level {
			(*inv)[i].Count++
			return
		}
	}
	*inv = append(*inv, Object{Type: t, Level: level, Count: 1})
}

// Decrement the counter for one object, and remove it from the list when count is zero
func (inv *PlayerInv) Remove(t ObjectCode, lvl uint32) {
	for i, obj := range *inv {
		if obj.Type != t || obj.Level != lvl {
			continue
		}
		(*inv)[i].Count--
		if obj.Count == 0 {
			inv.RemoveIndex(i)
		}
		return
	}
}

// Remove inventory entry 'index' completely from the list
func (inv *PlayerInv) RemoveIndex(index int) {
	if index > len(*inv) {
		log.Panicln("Illegal index", index, "for", inv)
	}
	last := len(*inv) - 1
	(*inv)[index] = (*inv)[last]
	*inv = (*inv)[0:last] // Reslice it, to drop last element
	return
}

// Use an object, return true if it was consumed
func (inv *PlayerInv) Use_WluBl(up *user, t ObjectCode, lvl uint32) {
	i := inv.Find(t, lvl)
	if i == -1 {
		up.Printf_Bl("#FAIL")
		return // No such object in inventory
	}
	f := objectUseTable[t]
	consumed, broadcast := f(up, t, lvl)
	if broadcast {
		ReportEquipmentToNear_Bl(up)
	}
	if consumed {
		ReportOneInventoryItem_WluBl(up, t, lvl)
	} else {
		up.Printf_Bl("#FAIL")
	}
}

// Used for debugging
func (this *PlayerInv) Report(wr io.Writer) {
	for _, o := range *this {
		fmt.Fprintf(wr, "!%#v", o)
	}
	return
}

// Compute the experience point value of dropping an item. The value is normalized, giving 1 for
// an item of the same level as the player and of the lowest grade.
// The type of the item has to be of the form "XXXN".
func ItemValueAsDrop(playerLevel, itemLevel uint32, t ObjectCode) float32 {
	if len(t) != 4 || t[3] > '9' || t[3] < '0' {
		return 0
	}
	itemType := t[3] - '0'
	diff := 1 + (float32(itemLevel)-float32(playerLevel)+float32(itemType))/2
	if diff < 0 {
		diff = 0
	} else if diff > 3 {
		diff = 3
	}
	return diff
}

func ConvertWeaponTypeToID(weapongrade uint8) ObjectCode {
	code := []byte(ItemWpnHandsID)
	code[3] += weapongrade
	return ObjectCode(code)
}

func ConvertArmorTypeToID(armorgrade uint8) ObjectCode {
	code := []byte(ItemArmorNoneId)
	code[3] += armorgrade
	return ObjectCode(code)
}

func ConvertHelmetTypeToID(helmetgrade uint8) ObjectCode {
	code := []byte(ItemHelmetNoneId)
	code[3] += helmetgrade
	return ObjectCode(code)
}

// Use a item of type 't' and level 'lvl'. The type can be counted on being 4 characters.
// Return first flag for being consumed, and teh second to broadcast the action to other players
func UsePotion_Wlu(up *user, t ObjectCode, lvl uint32) (consumed, broadcast bool) {
	pl := &up.player
	if pl.Dead {
		return false, false
	}
	switch t {
	case ItemHealthPotionID:
		up.Lock()
		// TODO: The amount should depend on the level
		if up.Heal(0.3, 0) {
			pl.Inventory.Remove(ItemHealthPotionID, lvl)
			consumed = true
		}
		up.Unlock()
	case ItemManaPotionID:
		up.Lock()
		// TODO: The amount should depend on the level
		if up.AddMana(0.3) {
			pl.Inventory.Remove(ItemManaPotionID, lvl)
			consumed = true
		}
		up.Unlock()
	}
	return
}

// Use a item of type 't' and level 'lvl'. The type can be counted on being 4 characters.
// Return first flag for being consumed, and teh second to broadcast the action to other players
func UseWeapon_Wlu(up *user, t ObjectCode, lvl uint32) (bool, bool) {
	replaced := false
	grade := t[3] - '0'
	pl := &up.player
	up.Lock()
	if pl.WeaponLvl+uint32(pl.WeaponGrade) < lvl+uint32(grade) {
		// Move the old item back to the inventory
		pl.Inventory.AddOneObject(ConvertWeaponTypeToID(pl.WeaponGrade), pl.WeaponLvl)
		// Update current item type
		pl.WeaponGrade = grade
		pl.WeaponLvl = lvl
		pl.Inventory.Remove(t, lvl)
		replaced = true
	}
	up.Unlock()
	return replaced, replaced
}

// Use a item of type 't' and level 'lvl'. The type can be counted on being 4 characters.
// Return first flag for being consumed, and teh second to broadcast the action to other players
func UseArmor_Wlu(up *user, t ObjectCode, lvl uint32) (bool, bool) {
	replaced := false
	grade := t[3] - '0'
	pl := &up.player
	up.Lock()
	if pl.ArmorLvl+uint32(pl.ArmorGrade) < lvl+uint32(grade) {
		// Move the old item back to the inventory
		pl.Inventory.AddOneObject(ConvertArmorTypeToID(pl.ArmorGrade), pl.ArmorLvl)
		// Update current item type
		pl.ArmorGrade = grade
		pl.ArmorLvl = lvl
		pl.Inventory.Remove(t, lvl)
		replaced = true
	}
	up.Unlock()
	return replaced, replaced
}

// Use a item of type 't' and level 'lvl'. The type can be counted on being 4 characters.
// Return first flag for being consumed, and teh second to broadcast the action to other players
func UseScroll_Wlu(up *user, t ObjectCode, lvl uint32) (consumed, broadcast bool) {
	pl := &up.player
	if pl.Dead {
		return
	}
	switch t {
	case ItemScrollRessID:
		up.Lock()
		pl.ReviveSP = pl.Coord
		pl.Inventory.Remove(t, lvl)
		up.Unlock()
		consumed = true
	}
	return
}

// Use a item of type 't' and level 'lvl'. The type can be counted on being 4 characters.
// Return first flag for being consumed, and teh second to broadcast the action to other players
func UseHelmet_Wlu(up *user, t ObjectCode, lvl uint32) (bool, bool) {
	replaced := false
	grade := t[3] - '0'
	pl := &up.player
	up.Lock()
	if pl.HelmetLvl+uint32(pl.HelmetGrade) < lvl+uint32(grade) {
		// Move the old item back to the inventory
		pl.Inventory.AddOneObject(ConvertHelmetTypeToID(pl.HelmetGrade), pl.HelmetLvl)
		// Update current item type
		pl.HelmetGrade = grade
		pl.HelmetLvl = lvl
		pl.Inventory.Remove(t, lvl)
		replaced = true
	}
	up.Unlock()
	return replaced, replaced
}
