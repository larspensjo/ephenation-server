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
// This is a general configuration file for the game engine
// Times are defined in nonoseconds. That means 1e8=0.1s, 1e9=1s, 1e10=10s, 1e11=100s, etc.
//
const (
	// How many nanoseconds between update of player and monster positions
	ObjectsUpdatePeriod         = 1e8       // 10 times per second update
	MonstersUpdateTargetPeriod  = 5e9       // How frequently the monster looks reevaluate when to switch target (aggro)
	MonstersUpdateDirPeriod     = 1e9       // How frequent the monster will evaluate what direction to move
	CnfgAttackPeriod            = 1e9       // How frequently attacks are updated, or healing if no attack
	CnfgAutosavePeriod          = 3e11      // Autosave players
	CnfgDefaultTriggerBlockTime = 10        // Default seconds until an activation block can be used again
	PURGE_MONSTER_PERIOD        = 3e10      // 30s second between every purge
	SPAWN_MONSTER_PERIOD        = 1e10      // 10s between tests
	CnfgHealingPeriod           = 1.2e11    // Nanoseconds needed for healing 100%
	MonsterMovingProb           = 0.6       // The probability that the monster is moving when no aggro
	MAX_PLAYERS                 = 2000      // Maximum number of logged in players
	RUNNING_SPEED               = 4.0       // blocks per second of a running player
	GRAVITY                     = 5.0       // blocks per seconds squared
	QuadtreeInitSize            = 64        // Measured in blocks.
	DefaultMonsterSpawnDistance = 25        // Number of blocks away that random monsters will spawn
	CheckMonsterDistForSpawn    = 40        // Monsters inside this distance are counted when deciding whether to spawn more
	CnfgMonsterAggroDistance    = 20        // How close you have to be to a monser to get aggro
	CnfgMonsterFieldOfView      = 1.40      // The viewing angle for a monster, in radians
	CnfgMeleeDistLimit          = 4         // Max block distance to be allowed to hit
	MaxMonsterSpawnHeightDiff   = 6         // The monster will not spawn outside of this diff to the player (in blocks)
	MonsterLimitForRespawn      = 3         // If there is at least this many monsters near, no new will be spawned.
	LoginChallengeLength        = 20        // The number of random bytes used in login, sent to the client.
	FlyingSpeedFactor           = 3         // How much quicker you fly than walking
	PlayerHeight                = 1.8 * 2   // Height of player 180 cm = 1.8m = 3.6 blocks
	PlayerJumpSpeed             = 2         // Number of blocks per seoond intial upward speed.
	WORLD_SOIL_LEVEL            = 9         // No soil above this level
	FLOATING_ISLANDS_LIM        = 96        // No floating islands are created below this level
	FLOATING_ISLANDS_PROB       = 0.85      // The density required for a floating island
	CnfgCaveWidth               = 0.1       // A bigger number will make cave tunnels wider
	CnfgTestPlayerNamePrefix    = "test"    // When test players are allowed, whithout password, the name has to begine with this string.
	CnfgTestPlayersPerChunk     = 1.0       // Defines the density of test players
	ClientChannelSize           = 100       // Number of messages that can wait for being sent
	CnfgManaForHealing          = 0.35      // Mana needed for the healing spell
	CnfgHealthAtHealingSpell    = 0.3       // How much the player heals for a healing spell
	CnfgManaForCombAttack       = 0.15      // Mana needed for combination attack
	CnfgWeaponDmgCombAttack     = 1.5       // Damage for the extra attack
	CnfgMaxOwnChunk             = 10        // The number of chunks a normal player can own. It can be overriden.
	CnfgJellyTimeout            = 15        // Number of seconds a block will be in jelly state
	CnfgItemRewardNormalizer    = 0.02      // How much experience to get for a dropped item of lowest grade at player level
	CnfgScoreMoveFact           = 1.0 / 128 // This means that a player need to move 64 blocs in a chunk to award 1 point
	CnfgScoreDamageFact         = 1.0 / 5   // Number of monsters that need to be killed for one point
	CnfgChunkFolder             = "DB"      // The folder where all chunks are stored
	CnfgSuperChunkFolder        = "SDB"     // The folder where all super chunks are stored
)
