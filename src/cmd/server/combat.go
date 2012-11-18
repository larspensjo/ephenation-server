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

//
// This file handles all combat algorithms
//

import (
	"client_prot"
	"math"
	// "fmt"
)

const (
	// Two constants the determnine how much damage is done depending on level difference. The current
	// tuning is set so as to increase a 30% more damage when opponent is decreased with one level.
	COMB_NumberOfHitsToKill = 10
	COMB_KillAdjust         = 0.262 // See https://docs.google.com/spreadsheet/ccc?key=0AnhOqCG9cmlvdHdlWmRDYkJRcklwOVZGNDh1alVIa3c
	// Three constants that determine the amount of experience points for a kill, depending on level difference
	COMB_ExpLevelOffset          = -0.7   // Used for sigmoid
	COMB_ExpMultiplier           = 1.5    // Multiplicative factor high lvl monster
	COMB_ExpForOneMonster        = 0.0083 // Amount of exp for killing a monster at the same level
	COMB_MonsterLevelFreeZoneHor = 32     // All monsters spawn within this horisontal distance will have level 0
	COMB_MonsterLevelVertFactor  = 3      // The vertical difficulty grows this times faster than the horisontal
	COMB_MonsterLevelGrowth      = 32     // Number of blocks you have to walk to meet monsters at the next level.
	COMB_MonsterVsPlayerFact     = 2      // Damage multiplier, used inverted in both directions
	COMB_EasyMonsterLevel        = 10     // The monsters are easier below this level.
	COMB_AggresiveLevelStart     = 5      // This is the level where monsters will start to get aggresive
	// If every armor item would give the same multiplier modifier as a weapon, then a list of top grade equipments
	// would totally dominate. A calibration constant is used to compensate for this. See computation at
	// https://docs.google.com/spreadsheet/ccc?key=0AnhOqCG9cmlvdHdlWmRDYkJRcklwOVZGNDh1alVIa3c&hl=en_US#gid=3
	COMB_ArmorModifierCal = 0.282
)

var (
	// Amount of experience for killing a monster at the same level as the player
	combatExperienceSameLevel = ExperienceForKill(0, 0)
)

// The difficult of a monster shall depend on the distance to a starting line.
func MonsterDifficulty(uc *user_coord) uint32 {
	dist := math.Abs(uc.Y) + math.Abs(uc.Z)*COMB_MonsterLevelVertFactor - COMB_MonsterLevelFreeZoneHor
	if dist <= 0 {
		return 0
	}
	return uint32(dist / COMB_MonsterLevelGrowth)
}

// The factor between monsters and players shall normally be COMB_MonsterVsPlayerFact, but more exaggerated for low level monsters.
// The purpose is to make it easy for beginners.
func MonsterVsPlayerFactor(level uint32) float32 {
	diff := COMB_EasyMonsterLevel - float32(level)
	if diff < 0 {
		diff = 0
	}
	return COMB_MonsterVsPlayerFact + float32(COMB_MonsterVsPlayerFact)/COMB_EasyMonsterLevel*diff
}

// The player is hit by a monster with the attributes as specified by the arguments
func (up *user) Hit(monster uint32, level uint32, weaponDmg float32, weaponLvl uint32) {
	dmg := weaponDmg *
		PlayerLevelDiffMultiplier(up.pl.Level, level) *
		WeaponLevelDiffMultiplier(level, weaponLvl, 1) /
		ArmorLevelDiffMultiplier(up.pl.Level, up.pl.ArmorLvl, up.pl.ArmorType) /
		ArmorLevelDiffMultiplier(up.pl.Level, up.pl.HelmetLvl, up.pl.HelmetType) /
		MonsterVsPlayerFactor(level)
	if dmg > 1 {
		dmg = 1
	}

	// This shouldn't be updated in the current process
	f := func(up *user) {
		up.Lock()
		up.pl.HitPoints -= dmg
		up.updatedStats = true // This leads to a message being generated.
		if up.pl.HitPoints <= 0 {
			up.pl.HitPoints = 0
			up.pl.Dead = true
			up.Unlock() // Must unlock before calling a function that can block
		} else {
			up.Unlock()
		}
		cp := ChunkFindCached_WLwWLc(up.pl.Coord.GetChunkCoord())
		owner := cp.owner
		if owner != up.pl.Id && owner != OWNER_NONE && owner != OWNER_RESERVED && owner != OWNER_TEST {
			up.AddScore(owner, float64(dmg)*CnfgScoreDamageFact)
		}
		var b [8]byte
		b[0] = 8
		b[1] = 0
		b[2] = client_prot.CMD_RESP_PLAYER_HIT_BY_MONSTER
		EncodeUint32(monster, b[3:7])
		b[7] = byte(dmg*255 + 0.5)
		up.writeBlocking_Bl(b[:])
	}
	up.SendCommand(f)
}

// The monster is hit by a player with the attributes as specified by the arguments
func (mp *monster) Hit_WLuBl(up *user, weaponDmg float32) {
	dmg := weaponDmg * PlayerLevelDiffMultiplier(mp.Level, up.pl.Level) * WeaponLevelDiffMultiplier(up.pl.Level, up.pl.WeaponLvl, up.pl.WeaponType) * MonsterVsPlayerFactor(mp.Level)
	if dmg > 1 {
		dmg = 1
	}
	mp.HitPoints -= dmg
	mp.updatedStats = true
	if mp.HitPoints <= 0 {
		pLevel := up.pl.Level
		mp.HitPoints = 0
		mp.dead = true
		experience := ExperienceForKill(pLevel, mp.Level)
		up.Lock()
		up.flags &= ^client_prot.UserFlagInFight
		up.pl.NumKill++
		// Give more experience to low level players
		switch up.pl.Level {
		case 0:
			experience *= 5
		case 1:
			experience *= 2.5
		case 2:
			experience *= 1.5
		}
		up.AddExperience(experience) // Must be locked
		up.Unlock()
		up.MonsterDropWLu(combatExperienceSameLevel / experience) // Adjust probability, relative
		// fmt.Printf("mp.Hit %#v\n", *mp)
	}
	var b [8]byte
	b[0] = 8
	b[1] = 0
	b[2] = client_prot.CMD_RESP_PLAYER_HIT_MONSTER
	EncodeUint32(mp.id, b[3:7])
	b[7] = byte(dmg*255 + 0.5)
	up.writeBlocking_Bl(b[:])
}

// Compare levels l1 and l2 of two fighters, and return a multiplier used in combat.
// Return a multiplier between 0 and 1.
func PlayerLevelDiffMultiplier(l1, l2 uint32) float32 {
	return float32(math.Exp(COMB_KillAdjust*(float64(l2)-float64(l1))) / COMB_NumberOfHitsToKill)
}

// Compensate for a fighter at level pLevel using a weapon at level wLevel.
// The principle is that a weapon at wrong level will give a penalty.
// Return a multiplier near 1. The same algorithm is used by the client in the inventory
// screen. If it is changed here, make sure it is also changed in the client.
// See https://docs.google.com/drawings/d/1cmObDuDgBvYhpsQagfjHPlckXjluU3CR9utVkTSh_rE/edit?hl=en_US
func WeaponLevelDiffMultiplier(pLevel, wLevel uint32, wepType uint8) (ret float32) {
	var detract uint8
	if pLevel > wLevel+uint32(wepType) {
		detract = uint8(pLevel-wLevel) - wepType // This will be greater than zero
	}
	if pLevel+uint32(wepType) < wLevel {
		detract = uint8(wLevel-pLevel) - wepType // This will be greater than zero
	}
	if detract > wepType {
		detract = wepType
	}
	// fmt.Println("WeaponLevelDiffMultiplier: detract ", detract)
	wepType -= detract
	switch wepType {
	case 0:
		ret = 0.9
	case 1:
		ret = 1.0
	case 2:
		ret = 1.1
	case 3:
		ret = 1.2
	case 4:
		ret = 1.3
	default:
		ret = 1.0 // Shouldn't happen
	}
	return
}

// Compensate for a fighter at level 'pLevel' using an armor at level 'aLevel'. This is also used
// for helmets.
// The same principle (but not same value) is used as for a weapon, see above. The damage is divided by this factor.
func ArmorLevelDiffMultiplier(pLevel, aLevel uint32, armType uint8) float32 {
	var detract uint8
	if pLevel > aLevel+uint32(armType) {
		detract = uint8(pLevel-aLevel) - armType // This will be greater than zero
	}
	if pLevel+uint32(armType) < aLevel {
		detract = uint8(aLevel-pLevel) - armType // This will be greater than zero
	}
	if detract > armType {
		detract = armType
	}
	// fmt.Println("WeaponLevelDiffMultiplier: detract ", detract)
	armType -= detract
	modifier := float32(1.0) // Default value
	switch armType {
	case 0:
		modifier = 0.9
	case 1:
		modifier = 1.0
	case 2:
		modifier = 1.1
	case 3:
		modifier = 1.2
	case 4:
		modifier = 1.3
	}
	return (modifier-1)*COMB_ArmorModifierCal + 1
}

// Calculate the experience for player 'pl' killing a monster at level 'level'
func ExperienceForKill(pLevel, mLevel uint32) float32 {
	return COMB_ExpForOneMonster * float32(COMB_ExpMultiplier/(1+math.Exp(float64(pLevel)-float64(mLevel)+COMB_ExpLevelOffset)))
}
