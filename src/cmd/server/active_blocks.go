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
// This file manages effects of active blocks.
//

package main

import (
	// "fmt"
	"client_prot"
	"keys"
	"log"
	"math"
	"quadtree"
	"score"
	"strconv"
	"strings"
	"time"
	. "twof"
)

// Nothing is locked when this function is called.
// Although chunks, players and monsters are accessed, a design goal is to not lock any at the same time.
// Because of that, extra effort is needed to copy, i.e. chunk data, before the next step.
func (up *user) CheckAndActivateTriggers_WLwWLuWLqWLmWLc(bl block) {
	ignoreTrigger := false
	// There is a theoretical chance that the following test can fail because of not using a lock.
	// If so, the worst concequence would be that the trap would fail to trig.
	if up.pl.Dead {
		ignoreTrigger = true
	}
	// A filter to prevent the same trigger from activating again
	switch bl {
	case BT_Trigger:
		if up.trapPrevBlock == BT_Trigger {
			ignoreTrigger = true
		}
	case BT_DeTrigger:
		if up.trapPrevBlock == BT_DeTrigger {
			ignoreTrigger = true
		}
	default:
		ignoreTrigger = true
	}
	up.trapPrevBlock = bl
	if ignoreTrigger {
		return
	}
	// Player moved into a trig block.
	// up.Printf_Bl("You moved into a trig block")

	// Normally, the player coordinate can not change from another process. If it would happen,
	// in worst case we would nto find the trap.
	coord := up.pl.Coord
	cc := coord.GetChunkCoord()
	cp := ChunkFindCached_WLwWLc(cc)
	x_off := uint8(int64(math.Floor(coord.X)) - int64(cc.X)*CHUNK_SIZE)
	y_off := uint8(int64(math.Floor(coord.Y)) - int64(cc.Y)*CHUNK_SIZE)
	z_off := uint8(int64(math.Floor(coord.Z)) - int64(cc.Z)*CHUNK_SIZE)

	// Iterate hrough all triggers, find the activator blocks and activate them. A local list of activators is
	// created, with the chunk read locked. Teh purpose of a copy in a local list is to be able to allow chunk to be unlocked.
	type localActivatorList struct { // Information about one activator
		msgList      []string // The list of strings for this activator
		x, y, z      uint8    // The local coordinate in the chunk
		index        int      // The index into the list of all activators
		inhibitDelta int      // Number of seconds until next inhibitor
	}
	list := make([]localActivatorList, 0, 5) // Allocate a number of pointers, to avoid unneccesary reallocation to grow the vector.
	now := time.Now()
	cp.RLock()
	ch_coord := cp.Coord
	owner := cp.owner
	for i, trig := range cp.blTriggers {
		if trig.x != x_off || trig.y != y_off || trig.z != z_off {
			continue // Wrong trigger
		}
		if cp.rc[trig.x2][trig.y2][trig.z2] != BT_Text {
			log.Println("Error: No text activator", cp.rc[trig.x2][trig.y2][trig.z2], trig)
			continue
		}
		msg := trig.msg
		if msg == nil {
			// This should never happen
			log.Println("Error: Nil message", ch_coord, trig, cp)
			continue
		}
		// Find the message associated with this activator
		if msg.inhibit.Before(now) {
			var la localActivatorList
			la.index = i
			la.msgList = trig.msg.Message
			la.x, la.y, la.z = trig.x2, trig.y2, trig.z2
			list = append(list, la)
		}
	}
	cp.RUnlock()

	// Now that there is a list of activators, possibly empty, the chunk no longer need to be locked.
	for i, _ := range list {
		msg := &list[i]
		var activatorCoord user_coord
		activatorCoord.X = float64(ch_coord.X)*CHUNK_SIZE + float64(msg.x)
		activatorCoord.Y = float64(ch_coord.Y)*CHUNK_SIZE + float64(msg.y)
		activatorCoord.Z = float64(ch_coord.Z)*CHUNK_SIZE + float64(msg.z)
		if *verboseFlag > 1 {
			log.Println("ActivateBlock", ch_coord, msg)
		}
		msg.inhibitDelta = CnfgDefaultTriggerBlockTime // Start with default, may be changed below
		// Iterate through each message for this activator
		for _, line := range msg.msgList {
			// Default is that the list of recpeients is only the current user. But it may change if
			// there is a /broadcast flag.
			recepients := []quadtree.Object{up}                                                                               // Assume only this player is involved
			newDeltaTime, terminate, recepients := up.ActivatorMessage_WLuWLqWLmWLc(line, &activatorCoord, recepients, owner) // Allow the list of recipient to change
			if terminate {
				// The rest of the data in this text activator shall be skipped
				break
			}
			if newDeltaTime != -1 {
				// There was a new inhibit defined, use it.
				msg.inhibitDelta = newDeltaTime
			}
		}
	}

	// Update the new inhibit time, if there was an activator that was used
	if len(list) > 0 {
		cp.Lock()
		for i, _ := range list {
			selected := &list[i]
			for j, _ := range cp.triggerMsgs {
				msg := &cp.triggerMsgs[j]
				if msg.X == selected.x && msg.Y == selected.y && msg.Z == selected.z {
					// Found it.
					msg.inhibit = now.Add(time.Duration(selected.inhibitDelta) * 1e9)
					break
				}
			}
		}
		cp.Unlock()
	}
}

// Iterate over all recepients, and call the provided function for each of them.
// Return the number of found players
func ActivatorIterator(f func(*user), list []quadtree.Object) (found int) {
	for _, recepient := range list {
		// For each object in the list, identify those that are players.
		if other, ok := recepient.(*user); ok && !other.pl.Dead {
			f(other)
			found++
		}
	}
	return
}

// Give the string in 'line', send it to every player in the 'recipients' list.
// Find special commands (with leading slash) in the string, and act accordingly.
// The following is returned:
// * The time, in seconds, until the next allowed activation. If there is no new inhibit defined, return -1.
// * A "terminate" flag, that means that the activator shall be ignored this time.
// * An updated recepients list.
func (up *user) ActivatorMessage_WLuWLqWLmWLc(line string, ac *user_coord, recepients []quadtree.Object, owner uint32) (inhibit int, terminate bool, updatedRecep []quadtree.Object) {
	// Initiallize all return parameters to default values
	inhibit = -1
	terminate = false
	updatedRecep = recepients
	for len(line) > 0 && line[0] == '/' {
		split := strings.SplitN(line, " ", 2)
		switch {
		case strings.HasPrefix(split[0], "/level>"):
			lim, err := strconv.Atoi(split[0][7:])
			terminate = err == nil && up.pl.Level <= uint32(lim)
			if terminate {
				return
			}
		case strings.HasPrefix(split[0], "/level<"):
			lim, err := strconv.Atoi(split[0][7:])
			terminate = err == nil && up.pl.Level >= uint32(lim)
			if terminate {
				return
			}
		case strings.HasPrefix(split[0], "/admin>"):
			lim, err := strconv.Atoi(split[0][7:])
			terminate = err == nil && up.pl.AdminLevel <= uint8(lim)
			if terminate {
				return
			}
		case strings.HasPrefix(split[0], "/keycond:"):
			// The rest of the line is consumed by this command
			descr := ""
			if len(split) > 1 {
				descr = split[1]
			}
			terminate = !TestKeyCond_RLu(up, owner, split[0][9:], descr)
			return
		case strings.HasPrefix(split[0], "/monster"):
			ActivatorMessageMonster_WLuWLqWLm(recepients, split[0][8:], ac)
		case strings.HasPrefix(split[0], "/invadd:"):
			newInhibit := ActivatorMessageInventoryAdd(recepients, split[0][8:], ac)
			if newInhibit != -1 && inhibit == -1 {
				inhibit = newInhibit
			}
		case strings.HasPrefix(split[0], "/broadcast:"):
			broadcastDistance, err := strconv.ParseFloat(split[0][11:], 64)
			if err == nil {
				if broadcastDistance > 20 {
					broadcastDistance = 20
				}
				// As it is a broadcast command, get list of all near players (will replace the default list of only the current user)
				updatedRecep = playerQuadtree.FindNearObjects_RLq(&TwoF{ac.X, ac.Y}, broadcastDistance)
			} else if *verboseFlag > 1 {
				log.Println("Broadcast error", err, "at", ac)
			}
		case strings.HasPrefix(split[0], "/inhibit:"):
			i, err := strconv.Atoi(split[0][9:]) // Measured in seconds
			if err != nil {
				log.Println("Inhibit error", err, "at", ac)
				i = -1
			}
			inhibit = i
		case strings.HasPrefix(split[0], "/addkey:"):
			// The rest of the line is consumed by this command
			descr := ""
			if len(split) > 1 {
				descr = split[1]
			}
			ActivatorMessageAddKey_WLu(recepients, owner, split[0][8:], descr)
			return
		case strings.HasPrefix(split[0], "/jelly:"):
			// Only one charater is expected.
			updatedRecep = playerQuadtree.FindNearObjects_RLq(&TwoF{ac.X, ac.Y}, client_prot.NEAR_OBJECTS)
			ActivatorMessageJellyblock_WLwWLc(updatedRecep, split[0][7:], *ac)
			if inhibit < CnfgJellyTimeout {
				// Do not allow open door again until it has closed
				inhibit = CnfgJellyTimeout
			}
		default:
			log.Println("Unknown modifier", split[0])
		}
		if len(split) > 1 {
			line = split[1]
		} else {
			line = ""
		}
	}
	// Whatever remains at this point is just text without modifiers
	if len(line) > 0 {
		ActivatorMessageString(recepients, line)
	}
	return
}

// This is a simple one, where we send the string to a recipient
func ActivatorMessageString(recepients []quadtree.Object, modifier string) {
	f := func(up *user) {
		up.Printf("%s", modifier)
	}
	ActivatorIterator(f, recepients)
}

// This is a simple one, where we send the string to a recipient
func ActivatorMessageJellyblock_WLwWLc(recepients []quadtree.Object, direction string, ac user_coord) {
	switch direction {
	case "n": // North
		ac.Y++
	case "w": // West
		ac.X--
	case "s": // South
		ac.Y--
	case "e": // East
		ac.X++
	case "u": // Up
		ac.Z++
	case "d": // Down
		ac.Z--
	}
	cc := ac.GetChunkCoord()
	cp := ChunkFindCached_WLwWLc(cc)
	x_off := uint8(int32(math.Floor(ac.X)) - cc.X*CHUNK_SIZE)
	y_off := uint8(int32(math.Floor(ac.Y)) - cc.Y*CHUNK_SIZE)
	z_off := uint8(int32(math.Floor(ac.Z)) - cc.Z*CHUNK_SIZE)
	cp.Lock()
	cp.TurnToJelly(x_off, y_off, z_off, time.Now().Add(CnfgJellyTimeout*1e9))
	cp.Unlock()

	// Compose the message that shall be sent to everyone near
	const length = 11
	var b [length]byte
	b[0] = length
	// b[1] = 0 // MSB of length
	b[2] = client_prot.CMD_JELLY_BLOCKS
	// b[3] = 0 // Flag
	b[4] = CnfgJellyTimeout
	b[5] = byte(cc.X & 0xFF)
	b[6] = byte(cc.Y & 0xFF)
	b[7] = byte(cc.Z & 0xFF)
	b[8] = x_off
	b[9] = y_off
	b[10] = z_off
	f := func(up *user) {
		up.writeNonBlocking(b[:])
	}
	ActivatorIterator(f, recepients)
}

// Test if a key is present, but only for the current user (regardless of /broadcast)
func TestKeyCond_RLu(up *user, owner uint32, modifier string, descr string) bool {
	// The expected modifier is: "A,B", where A is the key id and B is the owner id
	args := strings.SplitN(modifier, ",", 2)
	if len(args) != 2 {
		if *verboseFlag > 0 {
			log.Println("Bad key condition", args)
		}
		return false
	}
	keyId, err1 := strconv.ParseUint(args[0], 10, 0)
	ownerId, err2 := strconv.ParseUint(args[1], 10, 0)
	if err2 == nil {
		owner = uint32(ownerId)
	}
	if err1 == nil {
		up.RLock()
		res := up.pl.Keys.Test(owner, uint(keyId))
		up.RUnlock()
		if !res {
			up.Printf_Bl("%s", descr)
		}
		if *verboseFlag > 0 {
			log.Println("Cond key", owner, keyId, res)
		}
		return res
	} else {
		log.Println("Bad modifier", modifier, err1, err2)
	}
	return false
}

func ActivatorMessageAddKey_WLu(recepients []quadtree.Object, owner uint32, modifier string, name string) {
	f := func(up *user) {
		// The expected modifier is: "A,B", where A is the key id and B is the view
		args := strings.SplitN(modifier, ",", 2)
		if len(args) != 2 {
			if *verboseFlag > 0 {
				log.Println("Bad key", args)
			}
			return
		}
		keyId, err1 := strconv.ParseUint(args[0], 10, 0)
		viewId, err2 := strconv.ParseUint(args[1], 10, 0)
		if err1 == nil && err2 == nil {
			key := keys.Make(owner, uint(keyId), name, uint(viewId))
			up.Lock()
			up.pl.Keys = up.pl.Keys.Add(key)
			up.Unlock()
		} else {
			log.Println("Bad modifier", modifier, err1, err2)
		}
	}
	ActivatorIterator(f, recepients)
}

// Add an inventory item to all recepients. Return a new inhibit time, which depends on the reward
// value and number of recipients.
func ActivatorMessageInventoryAdd(recepients []quadtree.Object, modifier string, ac *user_coord) int {
	maker, ok := objectTable[modifier]
	if !ok {
		return -1
	}
	level := MonsterDifficulty(ac) // We want an object of a level corresponding to the monsters at this place.
	var quality float64
	if len(modifier) == 4 && modifier[3] <= '9' && modifier[3] >= '1' {
		quality = float64(modifier[3]) - '0' // A value 0 to 4.
	}
	cost := math.Pow(2, quality-1) // Will give a value of 0,5, 1, 2, 4, or 8
	f := func(up *user) {
		cp := ChunkFindCached_WLwWLc(up.pl.Coord.GetChunkCoord())
		owner := cp.owner
		costCovered := true
		if owner != OWNER_NONE && owner != OWNER_RESERVED && owner != OWNER_TEST && up.pl.Id != OWNER_TEST {
			// This time, also include the case where owner of chunk is up.
			costCovered = score.Pay(owner, cost)
		}
		if costCovered {
			obj := maker(level)
			AddOneObjectToUser_WLuBl(up, obj)
			// up.Printf("Added object %#v to %v", obj, up.pl.Name)
		}
	}
	numRecepients := ActivatorIterator(f, recepients)
	return CnfgDefaultTriggerBlockTime + (1+int(cost))*numRecepients*50
}

func ActivatorMessageMonster_WLuWLqWLm(recepients []quadtree.Object, modifier string, ac *user_coord) {
	deltaLevel := int32(0)
	switch modifier {
	case ":-1":
		deltaLevel = -1
	case ":+1":
		deltaLevel = 1
	case ":-2":
		deltaLevel = -2
	case ":+2":
		deltaLevel = 2
	case ":0":
		deltaLevel = 0
	default:
		log.Println("Unknown monster spawn modifier", modifier)
	}
	f := func(up *user) {
		mp := addMonsterToPlayerAtPos_WLuWLqWLm(up, ac, deltaLevel)
		mp.aggro = up
		mp.state = MD_ATTACKING
	}
	ActivatorIterator(f, recepients)
}

// Convert a chunk offset into something unique that can be used in a map.
func abKey(dx, dy, dz int) uint32 {
	return uint32(dx)<<16 + uint32(dy)<<8 + uint32(dz)
}

// Find all triggers in this chunk, follow the links, and find all connected activators. The chunk must be locked.
func (ch *chunk) ComputeLinks() {
	// Save the old list
	prevTextMsgActivator := ch.triggerMsgs
	// Make a new list, initialize length to the same as the old. This is the usual case, but need not be exactly correct.
	ch.triggerMsgs = make([]textMsgActivator, 0, len(prevTextMsgActivator))
	// fmt.Printf("ComputeLinks chunk %v\n", ch.Coord)
	ch.blTriggers = nil // Initialize to empty list
	for x := 0; x < CHUNK_SIZE; x++ {
		for y := 0; y < CHUNK_SIZE; y++ {
			for z := 0; z < CHUNK_SIZE; z++ {
				bl := ch.rc[x][y][z]
				if bl == BT_Text {
					// Add an empty trigger message for this text block.
					tm := textMsgActivator{uint8(x), uint8(y), uint8(z), nil, time.Unix(0, 0)}
					ch.triggerMsgs = append(ch.triggerMsgs, tm)
					// fmt.Println("ComputeLinks text at", tm)
					continue
				}
				if bl != BT_Trigger && bl != BT_DeTrigger {
					continue
				}
				// fmt.Println("ComputeLinks trigger at", x, y, z)
				tested := make(map[uint32]bool) // Create a new map to remember where we have been
				ch.FollowLinkNeighbors_WLwWLc(x, y, z, tested)
				// 0 or more activators have been added, but the trigger coord has to be updated afterwards
				for _, trigger := range ch.blTriggers {
					if trigger.x != CHUNK_SIZE {
						continue
					}
					trigger.x, trigger.y, trigger.z = uint8(x), uint8(y), uint8(z)
				}
			}
		}
	}
	CopyActivatorListMessages(prevTextMsgActivator, ch.triggerMsgs)
	ch.UpdateActivatorLinkMessages()
}

// The new activator list need to copy the mesages from the old activator list. After this
// copy, there may be new activation blocks with no messages, and some old activator messages that
// are no longer used. The old unused activator messages will not be copied, and so they will be discarded.
func CopyActivatorListMessages(sourceList, destList []textMsgActivator) {
	// Iterate through each of the new activators
	for i, dest := range destList {
		// Now find the old activator, if there was any
		for _, src := range sourceList {
			if dest.X == src.X && dest.Y == src.Y && dest.Z == src.Z {
				destList[i].Message = src.Message
				break
			}
		}
	}
}

// There are 6 possible neighbors that can continue the link
func (ch *chunk) FollowLinkNeighbors_WLwWLc(x, y, z int, tested map[uint32]bool) {
	ch.FollowLink_WLwWLc(x+1, y, z, tested)
	ch.FollowLink_WLwWLc(x, y+1, z, tested)
	ch.FollowLink_WLwWLc(x, y, z+1, tested)
	ch.FollowLink_WLwWLc(x-1, y, z, tested)
	ch.FollowLink_WLwWLc(x, y-1, z, tested)
	ch.FollowLink_WLwWLc(x, y, z-1, tested)
}

// Check the argument, as a neighbor chunk may be needed.
func (ch *chunk) FollowLink_WLwWLc(x, y, z int, tested map[uint32]bool) {
	// fmt.Printf("FollowLink new %v, orig %v\n", ch.Coord, orig.Coord)
	switch {
	case x < 0:
		fallthrough
	case y < 0:
		fallthrough
	case z < 0:
		fallthrough
	case x == CHUNK_SIZE:
		fallthrough
	case y == CHUNK_SIZE:
		fallthrough
	case z == CHUNK_SIZE:
		// Do not follow into next chunk. On the one hand, it would be nice to allow links
		// that span more than one chunk. But on the other hand, we can't lock more than
		// one chunk at a time, or risk dead lock.
		return
	}
	if tested[abKey(x, y, z)] {
		// This block has already been visited
		return
	}
	bl := ch.rc[x][y][z]
	if bl == BT_Spawn || bl == BT_Text {
		// Found an activator.
		// fmt.Printf("FollowLink: Activator %d at %d,%d,%d (chunk %v,)\n", bl, x, y, z, ch.Coord)
		var bt BlockTrigger
		bt.x2, bt.y2, bt.z2 = uint8(x), uint8(y), uint8(z)
		bt.x = CHUNK_SIZE // Illegal value to indicate that the trigger coordinate shall be updated later.
		ch.blTriggers = append(ch.blTriggers, &bt)
	} else if bl != BT_Link && bl != BT_Trigger {
		return // Dead end, this was not a link block
	}
	// Other triggers and spawners are also treated as a link
	// Current block type is regarded as a link
	tested[abKey(x, y, z)] = true
	ch.FollowLinkNeighbors_WLwWLc(x, y, z, tested)
}

// Find the activator messages for the given coordinate.
// The chunk must be appropiately locked.
func (ch *chunk) FindActivator(x, y, z uint8) *[]string {
	for i, act := range ch.triggerMsgs {
		if act.X == x && act.Y == y && act.Z == z {
			// Found it.
			return &ch.triggerMsgs[i].Message
		}
	}
	return nil
}

// For each trigger association, find the corresponding trigger message (if any).
// The chunk must be locked.
func (ch *chunk) UpdateActivatorLinkMessages() {
	// Loop through all trigger-activator connectsion
	for _, trig := range ch.blTriggers {
		// For each trigger, find the corresponding message
		for i := range ch.triggerMsgs {
			if trig.x2 == ch.triggerMsgs[i].X && trig.y2 == ch.triggerMsgs[i].Y && trig.z2 == ch.triggerMsgs[i].Z {
				trig.msg = &ch.triggerMsgs[i] // Found it, set the pointer
				break                         // Stop search for this one
			}
		}
	}
}
