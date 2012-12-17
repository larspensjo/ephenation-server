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
// Manage all monsters.
//
// The number of monsters can be huge, which means it is not effective
// to have them in an array. Instead, there are two linked lists; one
// list of active monsters, and one list for re-use.
//

import (
	"client_prot"
	. "twof"
	//"fmt"
	sync "github.com/larspensjo/Go-sync-evaluation/evalsync"
	"log"
	"math"
	"math/rand"
	"quadtree"
	"time"
)

const (
	// Monster speed attributes
	WALKING_FACTOR = 0.50 // The speed factor from maximum speed to use as a normal walk
	RUNNING_FACTOR = 0.75
	ASSAULT_FACTOR = 1.00 // Used only so that it is easy to expand the concept to include panic etc
	PANIC_FACTOR   = 1.10
)

type MonsterState uint8

// Monster state definitions
const (
	MD_NORMAL     = MonsterState(iota)
	MD_STROLLING  = MonsterState(iota)
	MD_TURNING    = MonsterState(iota)
	MD_HOSTILE    = MonsterState(iota)
	MD_ATTACKING  = MonsterState(iota)
	MD_DEFENDING  = MonsterState(iota)
	MD_PURSUING   = MonsterState(iota)
	MD_RECOVERING = MonsterState(iota)
	MD_GOHOME     = MonsterState(iota)
)

// A monster is not locked when data is manipulated. There is a risk that conflicting operations will be done
// at the same time, but it is unlikely and not fatal.
type monster struct {
	id uint32 // An ID for the client to refer to.

	Level       uint32  // The level of the monster
	HitPoints   float32 // HP of the monster, from 0 to 1
	size        float32 // How high the monster is
	maxSpeed    float32 // Maximum speed
	persistence float32 // How long a monster will keep on chasing a target
	aggression  float32 // How aggressive the monster is

	Coord      user_coord // The current monster coordinates
	prevCoord  user_coord // Keep track of when moving
	spawnCoord user_coord // Sometimes the monster choose to go "home".
	ZSpeed     float64    // Speed upwards.
	dirHor     float32    // The horistonal direction the monster is looking at. 0 radians means looking to the east.

	turningDir float32 // Turning direction for monsters that are turning away from an obstacle

	state MonsterState // Monster state

	aggro   *user   // The user that has aggro. The monster will follow it, and attack it when possible.
	speed   float32 // Current speed
	fatigue float32 // How tired the monster is

	mvFwd        bool // Flags if monster is moving. If it is moving, it is in the direction it is looking.
	hasMoved     bool // Flag if the monster has moved and noone has been told about it.
	purge        bool // True if this monster shall be removed
	invalid      bool // True if this monster is no longer valid. This is used if there would be other references, not using the map.
	dead         bool // True if this monster is dead
	updatedStats bool // Information about this monster has changed, and nearby clients need to know
}

// This struct manages a set of monsters. There is only one semaphore for the whole struct, which means only a limited number
// monsters can be handled as it will be serialized to one process only.
type GroupOfMonsters struct {
	nextId        uint32              // This is the next monster ID to be used. Wrap around, if ever, is handled.
	lastTimeMoved time.Time           // The time when the monsters were last moved
	m             map[uint32]*monster // Sparse array of all monsters
	sync.RWMutex                      // Provide a lock for accessing this structure
}

// There can only be one instance of the monster quadtree.
var monsterQuadtree = quadtree.MakeQuadtree(lowerLeftNearCorner, upperLeftFarCorner, 1)

var monsterData GroupOfMonsters

func init() {
	monsterData.m = make(map[uint32]*monster)
	monsterData.lastTimeMoved = time.Now()
}

func (m *monster) GetZ() float64 {
	return m.Coord.Z
}

func (m *monster) GetPreviousPos() *TwoF {
	return &TwoF{m.prevCoord.X, m.prevCoord.Y}
}

func (m *monster) GetType() uint8 {
	return client_prot.ObjTypeMonster
}

func (m *monster) GetId() uint32 {
	return m.id
}

// Get looking direction in radians
func (this *monster) GetDir() float32 {
	return this.dirHor
}

// This is the only function that control the monsters.
// TODO: More processes could be needed when there are many monsters.
func ManageMonsters_WLwWLuWLqWLmBlWLc() {
	go ProcSpawnMonsters_WLwWLuWLqWLmBlWLc()
	go ProcUpdateMonsterState()
	go ProcUpdateMonsterPos_RLmWLwWLqBlWLuWLc()
	go ProcUpdateMonstersTarget_RLmRLqBl()
	// go ProcUpdateMonstersDir_RLmBlRLu()
	go ProcMonsterMelee_RLmBl()
	ProcPurgeMonsters_WLmWLqBl() // Will not return
}

func UpdateAllMonstersState() {
	monsterData.Lock() // Write Lock monster data
	// fmt.Printf("Monster State Update\n")

	for _, mp := range monsterData.m {
		if mp.dead {
			continue
		}
		switch mp.state {
		case MD_NORMAL:
			//#fmt.Printf("Monster %v in state MD_NORMAL\n", mp.id)
			// TODO: Add GO HOME state
			if rand.Float32() > MonsterMovingProb {
				// Update dir as a function of previous dir, in order to avoid sharp turns
				// TODO: This is too small
				mp.dirHor += (0.5 - rand.Float32()) * math.Pi / 60 // 30 degrees
				if mp.dirHor < 0.0 {
					mp.dirHor += math.Pi * 2
				}
				if mp.dirHor > math.Pi*2 {
					mp.dirHor -= math.Pi * 2
				}
				mp.mvFwd = true
				mp.speed = WALKING_FACTOR * mp.maxSpeed
				mp.state = MD_STROLLING
			}

		case MD_STROLLING:
			//#fmt.Printf("Monster %v in state MD_STROLLING\n", mp.id)
			// TODO: Monster will not turn once it has started moving
			if rand.Float32() <= MonsterMovingProb {
				mp.mvFwd = false
				mp.state = MD_NORMAL
			}

		case MD_TURNING:
			//fmt.Printf("Monster %v in state MD_TURNING, dir: %v delta: %v\n", mp.id, mp.dirHor, mp.turningDir)
			mp.dirHor += mp.turningDir
			// After turning, try to move again. If not possible, this monster will go back to turning
			mp.mvFwd = true
			mp.speed = WALKING_FACTOR * mp.maxSpeed
			mp.state = MD_STROLLING

		case MD_HOSTILE:
			//#fmt.Printf("Monster %v in state MD_HOSTILE\n", mp.id)

		case MD_ATTACKING:
			//#fmt.Printf("Monster %v in state MD_ATTACKING\n", mp.id)
			up := mp.aggro
			if up == nil {
				//#fmt.Printf("Monster %v INVALID state MD_ATTACKING\n", mp.id)
				mp.speed = WALKING_FACTOR * mp.maxSpeed
				mp.state = MD_NORMAL
				break
			}
			up.RLock()
			if up.connState != PlayerConnStateIn || up.Dead {
				mp.aggro = nil
				// The player disappeared, lower speed to walk
				mp.speed = WALKING_FACTOR * mp.maxSpeed
				mp.state = MD_NORMAL
			} else {
				var dist2 float64
				// TODO: Check if turned in the right direction
				tempDirHor, dist2 := mp.ComputeDir(&up.player)
				mp.dirHor = tempDirHor

				/*
					if math.Fabs( float64(mp.dirHor - tempDirHor) ) < (math.Pi/12) {
						mp.dirHor = tempDirHor
					} else if mp.dirHor > tempDirHor  {
						mp.dirHor -= math.Pi / 12
					} else if mp.dirHor < tempDirHor {
						mp.dirHor += math.Pi / 12
					}

					if mp.dirHor < 0.0 {
						mp.dirHor += math.Pi * 2
					}
					if mp.dirHor > math.Pi * 2 {
						mp.dirHor -= math.Pi * 2
					}

					fmt.Printf("Monster %v heading %v\n", mp.id, mp.dirHor);
				*/

				//fmt.Printf("Monster %v: Fatigue:%v Aggro:%v\n", mp.id, mp.fatigue, mp.aggro.Name); // DEBUG
				// Since the function is called every MonstersUpdateDirPeriod ns (currently once per second), update the fatigue accordingly
				// Target for this is 15 secs for a "normal" attack, i.e. an animal with 100% percistance
				mp.fatigue -= mp.persistence / (15e9 / MonstersUpdateDirPeriod)

				if mp.fatigue <= 0.0 {
					// fmt.Printf("Monster %v: Abort attack on:%v\n", mp.id, mp.aggro.Name); // DEBUG
					mp.aggro = nil // Abort attack after this sequence
					mp.fatigue = 0.0
					mp.state = MD_RECOVERING
				}

				// Continue moving until inside melee distance
				if dist2 > CnfgMeleeDistLimit*CnfgMeleeDistLimit {
					mp.mvFwd = true
				} else {
					mp.mvFwd = false // Don't go too close to the player that has aggro
				}
			}
			up.RUnlock()

		case MD_DEFENDING:
			// TODO: When defending, change to attacking if the monster is "strong"
			// otherwise the monster may decide to "run away"
			// Currently the monster will automatically move to attacking
			//#fmt.Printf("Monster %v in state MD_DEFENDING\n", mp.id)
			mp.state = MD_ATTACKING

		case MD_RECOVERING:
			//#fmt.Printf("Monster %v in state MD_RECOVERING\n", mp.id)
			// TODO: Add a health recovery function as well?
			mp.fatigue += mp.persistence / (30e9 / MonstersUpdateDirPeriod) // Recovery time to full recovery is 30 secs
			// TODO: Does not need to be this, could also decide to return home
			// TODO: Could start moving, but maybe not attacking when in RECOVERY
			// TODO: Move away from players when recovering
			if mp.fatigue > 100.0 {
				mp.fatigue = 100.0
				mp.state = MD_NORMAL
			}

		default:
			// This is an unknown state, return to normal
			mp.state = MD_NORMAL
		}

		// TODO:Update persistence in certain states
	}

	monsterData.Unlock() // Unlock monster data
}

func UpdateAllMonstersTarget_RLmRLq() {
	// TODO: Integrate this function into the state machine
	monsterData.RLock()
	for _, mp := range monsterData.m {
		if mp.aggro != nil || mp.dead {
			continue // No change for this monster
		}
		nearObjects := playerQuadtree.FindNearObjects_RLq(mp.GetPreviousPos(), CnfgMonsterAggroDistance)
		if nearObjects == nil {
			mp.aggro = nil
			continue
		}
		// TODO: Test for aggro level (randomize attack)
		// TODO: Test for health
		// TODO: Test for persistence
		// TODO: Test for valid modes before attacking - more states needed
		if mp.state == MD_RECOVERING {
			continue
		}

		// TODO: Rather than not attacking players with too high level, decrease chance of an attack?
		// Temporary solution
		if 100.0*rand.Float32() > mp.aggression {
			//#fmt.Printf("M:%d Not aggressive\n",mp.id)
			continue
		}

		//#fmt.Printf("M:%d Aggro task\n",mp.id)
		for _, o := range nearObjects {
			up, ok := o.(*user)
			if !ok || up.Dead {
				continue // Only aggro on alive players
			}
			// Aggro on first near player found. TODO: Do something more sophisticated
			dir, dist2 := mp.ComputeDir(&up.player)
			deltaDir := math.Abs(float64(dir - mp.dirHor))
			if deltaDir > math.Pi {
				deltaDir = math.Abs(deltaDir - 2*math.Pi)
			}
			//#fmt.Printf("M:%d player dir %v moster heading %v diff %v\n",mp.id,dir,mp.dirHor,deltaDir)
			if dist2 < CnfgMonsterAggroDistance*CnfgMonsterAggroDistance {
				// TODO: This test is needed because of a problem with playerQuadtree.FindNearObjects not
				// taking into account the z axis.
				if up.Level > mp.Level+3 || mp.Level < COMB_AggresiveLevelStart {
					// The player is too high level. Simply turn the monster in the direction of the player,
					// but don't set aggro, and don't move it. That way, it will seem as if the monster now
					// and then is staring at the player.
					//#fmt.Printf("Level too high? M:%d P:%d\n", mp.Level, up.Level)
					mp.dirHor = dir
					mp.mvFwd = false
				} else if deltaDir < CnfgMonsterFieldOfView { // Make sure that the monster can "see" the player
					// Send a message to tell the client that the player has aggro from this monster
					var b [7]byte
					b[0] = 7
					b[1] = 0
					b[2] = client_prot.CMD_RESP_AGGRO_FROM_MONSTER
					EncodeUint32(mp.id, b[3:7])
					up.writeNonBlocking(b[:])
					mp.aggro = up
					mp.state = MD_ATTACKING
					// The monster should chase the player
					mp.speed = ASSAULT_FACTOR * mp.maxSpeed // Accelerate monster in aggro mode!
					//fmt.Printf("Monster %v: Speed:%v\n", mp.id, mp.speed)
				}
				break
			}
		}
	}
	monsterData.RUnlock()
}

// Compute the direction and distance (squared) the monster 'mp' should use to face a player 'up'
func (mp *monster) ComputeDir(pl *player) (dir float32, dist2 float64) {
	dx := pl.Coord.X - mp.Coord.X
	dy := pl.Coord.Y - mp.Coord.Y
	dz := pl.Coord.Z - mp.Coord.Z
	if dy == 0 {
		if dx > 0 {
			dir = math.Pi / 2
		} else {
			dir = -math.Pi / 2
		}
	} else {
		dir = float32(math.Atan2(dx, dy))
		// fmt.Printf("UpdateAllMonstersDir dx,dy (%.3f,%.3f) angle %f\n", dx, dy, mp.dirHor / 2 / math.Pi * 360)
	}
	if dir < 0 {
		dir += 2 * math.Pi // Avoid negative angles (will be a problem for the protocol with the client
	}
	dist2 = dx*dx + dy*dy + dz*dz
	return
}

// Manage melee
func CmdMonsterMelee_RLm() {
	monsterData.RLock()
	for _, mp := range monsterData.m {
		up := mp.aggro
		if up == nil || mp.dead {
			continue
		}
		if up.connState != PlayerConnStateIn || up.Dead {
			mp.aggro = nil
			continue
		}
		dx := up.Coord.X - mp.Coord.X
		dy := up.Coord.Y - mp.Coord.Y
		dz := up.Coord.Z - mp.Coord.Z
		dist := dx*dx + dy*dy + dz*dz // Skip the square root
		if dist > CnfgMeleeDistLimit*CnfgMeleeDistLimit {
			if dist > CnfgMonsterAggroDistance*CnfgMonsterAggroDistance {
				// Player is too far away to hold aggro.
				mp.aggro = nil
			}
			continue // Not near enough to be able to hit
		}
		up.Hit(mp.id, mp.Level, 1, mp.Level)
	}
	monsterData.RUnlock()
}

// To minimize new array allocates, this parameter is global.
var monstersPurged []*monster

// Find all monsters not close to any players and de-spawn them
func CmdPurgeMonsters_WLmWLq() {
	// Use a read lock to first identify monsters for purging. This will not
	// modify the table of monsters (which is what is locked). It will
	// only modify the monster itself.
	monsterData.RLock()
	for _, mp := range monsterData.m {
		if mp.dead {
			mp.purge = true
			continue
		}
		// Only remove monsters where there are no near players
		nearObjects := playerQuadtree.FindNearObjects_RLq(mp.GetPreviousPos(), client_prot.NEAR_OBJECTS)
		// fmt.Printf("PurgeMonstersCommand: Near objects to old monster at %v: %v\n", mp.Coord, nearObjects)
		foundNearPlayer := false
		for _, o := range nearObjects {
			_, ok := o.(*user)
			if ok {
				foundNearPlayer = true
				break
			}
		}
		if !foundNearPlayer {
			mp.purge = true
		}
	}
	monsterData.RUnlock()

	// Remove all purged monsters. This is quicker, but a write lock is needed
	// TODO: save the allocated monster blocks in a linked list, and re-use them
	// when needed. That will lessen the need for garbage collection
	monsterData.Lock()
	for i, m := range monsterData.m {
		if m.purge {
			monstersPurged = append(monstersPurged, m)
			delete(monsterData.m, i)
			continue
		}
	}
	monsterData.Unlock()

	// All purged monsters have to be removed form the quadtree. This is done outside of the lock of the monsterdata,
	// so that two locks wouldn't be needed at the same time.
	// log.Printf("List of purged monsters: %v\n", monstersPurged)
	for _, m := range monstersPurged {
		monsterQuadtree.Remove_WLq(m)
		m.invalid = true
	}
	monstersPurged = monstersPurged[0:0]
	// log.Printf("list after clearing: %v, len %d, cap %d\n", monstersPurged, len(monstersPurged), cap(monstersPurged))
}

func CountNearMonsters_RLq(pos *TwoF) int {
	near := monsterQuadtree.FindNearObjects_RLq(pos, CheckMonsterDistForSpawn)
	count := 0
	for _, o := range near {
		mp, ok := o.(*monster)
		if ok && !mp.dead {
			count++
		}
	}
	return count
}

// Try to find a place where a monster can be added to the specified player.
func addMonsterToPlayer_WLwWLuWLqWLmWLc(up *user) {
	if up.connState != PlayerConnStateIn {
		return
	}
	coord := up.Coord
	pos := &TwoF{coord.X, coord.Y}
	// Do not create too many monsters near this player.
	if CountNearMonsters_RLq(pos) >= MonsterLimitForRespawn {
		return
	}
	// There is a chance that the player moved, disconnected or logged out
	// during the following operations. In worst case, a monster will be spawned
	// at the wrong place, but that is acceptable.
	a, b := math.Sincos(rand.Float64() * math.Pi * 2)
	dx := b * DefaultMonsterSpawnDistance
	dy := a * DefaultMonsterSpawnDistance
	coord.X += dx
	coord.Y += dy
	// And do not create too many monsters at the new monster position.
	pos = &TwoF{coord.X, coord.Y}
	if CountNearMonsters_RLq(pos) >= MonsterLimitForRespawn {
		return
	}
	// Now try to find a z that is near the player and also an empty position.
	found := false
	var dz float64
	for dz = -MaxMonsterSpawnHeightDiff; dz < MaxMonsterSpawnHeightDiff; dz++ {
		// TODO: Not all monsters are 3 blocks high.
		if ValidSpawnPoint_WLwWLc(user_coord{coord.X, coord.Y, coord.Z + dz}, 3) {
			coord.Z += dz
			found = true
		}
	}
	if !found {
		// Couldn't find a valid spawn point.
		return
	}

	// Check if chunk is owned by someone, in which case no monster should be spawned
	// Owner OWNER_NONE is the "world" and owner OWNER_RESERVED is the starting area
	cc := coord.GetChunkCoord()
	cp := ChunkFind_WLwWLc(cc)
	if (cp.owner == OWNER_NONE) || (cp.owner == OWNER_RESERVED) || (cp.owner == OWNER_TEST) {
		addMonsterToPlayerAtPos_WLuWLqWLm(up, &coord, 0)
	}
}

// This is the second part, where a monster is added to a player.
func addMonsterToPlayerAtPos_WLuWLqWLm(up *user, coord *user_coord, deltaLevel int32) *monster {
	baseLevel := MonsterDifficulty(coord)
	lvl := int32(baseLevel) + deltaLevel
	if lvl < 0 {
		// Sanity check
		lvl = 0
	}
	m := new(monster)
	m.Coord = *coord
	m.prevCoord = m.Coord
	m.spawnCoord = *coord // The same as the initial coordinate
	m.Level = uint32(lvl)
	m.HitPoints = 1.0
	m.ZSpeed = 0
	m.dirHor = rand.Float32() * math.Pi * 2 // initial heading direction

	// The monster size algorithm is known by the client. If it is changed, it has to be changed at both places.
	rnd := (lvl + 137) * 871        // pseudo random 32-bit number
	rnd2 := float32(rnd&0xff) / 255 // Random number 0-1
	rnd2 *= rnd2
	rnd2 *= rnd2
	m.size = 1 + rnd2*4 // The monster size will range from 1.5 blocks to 5.5 blocks

	// Init monster data used to determine special traits
	m.state = MD_NORMAL
	m.maxSpeed = RUNNING_SPEED // Running speed of player TODO: Add variation, from slower than a player to faster
	m.maxSpeed *= -m.size*0.1 + 1.35
	m.speed = WALKING_FACTOR * m.maxSpeed // Start in walking mode
	m.persistence = 100.0                 // Persistence
	m.fatigue = 100.0                     // 100% is the starting fatigue, when this gets close to 0 the monster will stop chasing
	m.aggression = 50.0 + float32(m.Level%5)*10.0

	//#fmt.Printf("NEW M: lvl %d siz %.1f per %.1f agg %.1f spd %.1f\n", lvl, m.size, m.persistence, m.aggression, m.maxSpeed)

	// Find all near players (will be at least one), and tell them about the new monster.
	pos := &TwoF{m.Coord.X, m.Coord.Y}
	near := playerQuadtree.FindNearObjects_RLq(pos, client_prot.NEAR_OBJECTS)
	// fmt.Printf("Near objects to new player at %v: %v\n", up.Coord, near)
	for _, o := range near {
		up, ok := o.(*user)
		if !ok {
			continue // Only need to tell players, not monsters etc.
		}
		// Tell this player that the monster moved. Usually no lock needed for this
		// case, but we really don't want the client to miss that a new monster was added
		up.Lock()
		up.SomeoneMoved(m)
		up.Unlock()
	}
	monsterQuadtree.Add_WLq(m, pos)
	{
		// This region, where the monster is added, must be locked.
		// The following algorithm will succeed eventually, as the ID is a 32-bit number.
		monsterData.Lock()
		id := monsterData.nextId
		for ; ; id++ {
			_, ok := monsterData.m[id]
			if !ok {
				// Found a free entry
				break
			}
		}
		m.id = id
		monsterData.nextId = id + 1 // Will wrap around eventually, which is ok
		monsterData.m[id] = m       // Remember that this ID is in use
		monsterData.Unlock()
	}

	// fmt.Printf("New Monster %v: Lvl:%v Size:%v Speed:%v\n", m.id, m.Level, m.size, m.speed) // DEBUG

	return m
}

// Walk through all players and add monsters to players where needed
func CmdSpawnMonsters_WLwWLuWLqWLmWLc() {
	for i := 0; i < MAX_PLAYERS; i++ {
		up := allPlayers[i]
		if up != nil {
			addMonsterToPlayer_WLwWLuWLqWLmWLc(up)
		}
	}
}

// Move all monsters.
// TODO: Telling near clients about the state change should be managed from a separate process. This will save time
// as well as move handling of a logic elsewhere that isn't about moving only.
func UpdateAllMonsterPos_RLmWLwWLqWLuWLc(fullReport bool) {
	monsterData.RLock()
	now := time.Now()
	deltaTime := now.Sub(monsterData.lastTimeMoved)
	monsterData.lastTimeMoved = now
	if deltaTime > ObjectsUpdatePeriod*4 {
		// Sanity check. Monster should not move too long, unless deltaTime is big because of a PC sleep mode
		// or something like that. It an also happen with temporary high loads.
		if *verboseFlag > 1 {
			// TODO: This seems to happen a lot on the server. A common time is 0.5s and more. Worst case was 104s.
			// It seems some monsters are not pruned
			log.Printf("Unexpected long monster movement deltaTime %.3fs\n", float64(deltaTime)/1e9)
		}
		deltaTime = ObjectsUpdatePeriod * 2
	}

	// TODO: Monsters should avoid moving in a direction that will cause them to fall.
	// TODO: Monsters should avoid moving into water
	for _, m := range monsterData.m {
		oldCoord := m.Coord
		if !m.dead {
			m.Move_WLwWLc(deltaTime)
		}
		newPosition := m.Coord.X != oldCoord.X || m.Coord.Y != oldCoord.Y || m.Coord.Z != oldCoord.Z
		if fullReport || newPosition || m.updatedStats {
			m.updatedStats = false
			// Find near players and tell them 'm' has moved
			near := playerQuadtree.FindNearObjects_RLq(m.GetPreviousPos(), client_prot.NEAR_OBJECTS)
			// fmt.Printf("Near objects to %v: %v\n", up.Coord, near)
			// fmt.Println("ClientUpdatePlayerPosCommand OCTREE: ", playerQuadtree)
			for _, o := range near {
				if other, ok := o.(*user); ok {
					// Don't tell 'other' immediately, but queue it up
					other.Lock()
					other.SomeoneMoved(m)
					other.Unlock()
				}
			}
			if newPosition {
				monsterQuadtree.Move_WLq(m, &TwoF{m.Coord.X, m.Coord.Y})
				m.prevCoord = m.Coord
			}
		}
	}
	monsterData.RUnlock()
}

// Move the monster, if it wants to.
func (mp *monster) Move_WLwWLc(deltaTime time.Duration) {
	mp.ZSpeed = UpdateZPos_WLwWLc(deltaTime, mp.ZSpeed, &mp.Coord)
	// fmt.Println("New monster falling speed ", mp.ZSpeed, " at pos ", mp.Coord.Z)
	if !mp.mvFwd {
		return // Monsters aren't strafing, only moving in the direction they are looking
	}
	coord := mp.Coord
	s, c := math.Sincos(float64(mp.dirHor))
	dist := float64(mp.speed) * float64(deltaTime) / 1e9
	// fmt.Println("Monster move ", dist)
	dx := s * dist
	dy := c * dist
	coord.X += dx
	coord.Y += dy
	if blockIsPermeable[DBGetBlock_WLwWLc(coord)] {
		// TODO: Should check up to monster height
		mp.Coord = coord
		mp.updatedStats = true
		return
	}

	// There is some obstacle. Allow the movement if it is one block of height difference.
	coord.Z += 1
	if blockIsPermeable[DBGetBlock_WLwWLc(coord)] {
		mp.Coord = coord
		mp.updatedStats = true
		return
	}

	// TODO: There is a wall, and monster should follow the wall instead of stopping.
	mp.turningDir = math.Pi / 12 // 15 degrees TODO: Fix random component
	// TODO: Monsters will only turn in one direction, allow both
	//if rand.Float32() > 0.5 {
	//	mp.turningDir = -mp.turningDir
	//}
	mp.mvFwd = false
	mp.state = MD_TURNING
	//fmt.Printf("Setting state to TURNING for monster %v\n", mp.id)
}
