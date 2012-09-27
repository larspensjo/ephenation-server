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

// This file contains global functions, not called from the client proc, that use the "user" struct.

import (
	"chunkdb"
	"client_prot"
	"fmt"
	"inventory"
	"log"
	"math"
	"quadtree"
	. "twof"
)

func SaveAllPlayers_RLa() {
	allPlayersSem.RLock()
	for i := 0; i < MAX_PLAYERS; i++ {
		up := allPlayers[i]
		if up != nil && up.lic != nil && up.connState == PlayerConnStateIn {
			up.forceSave = true
		}
		// up.Printf("Autosave");
	}
	allPlayersSem.RUnlock()
}

// Send a text message to a player, which must not be locked.
// If the message can't be sent, discard it.
func (up *user) Printf(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	length := len(str) + 3
	prefix := []byte{byte(length & 0xFF), byte(length >> 8), client_prot.CMD_MESSAGE}
	msg := string(prefix) + str
	up.writeNonBlocking([]byte(msg))
}

// Send a message to a client, but it must not block. Because of that, send the message to the
// local client process that can handle the blocking.
// Condition: There is no guarantee to be any locks, which means there is no guarantee in what order
// these messages arrive to the client. Ok, the order will stay the same, but there may be other
// processes that manage to inject message in between others.
func (up *user) writeNonBlocking(b []byte) {
	if length := int(b[0]) + int(b[1])<<8; length != len(b) {
		panic("Wrong length of message")
	}
	if *verboseFlag > 2 {
		log.Printf("Non blocking Send to %v '%v'\n", up.pl.name, b)
	}
	select {
	case up.channel <- b:
	default:
	}
}

// Send a non blocking command to the player.
// The command can fail if the receiver is full.
func (up *user) SendCommand(cmd ClientCommand) {
	// Use a select statement to make sure it never blocks.
	select {
	case up.commandChannel <- cmd:
	default:
	}
}

func (this *user) SomeoneMoved(o quadtree.Object) {
	this.objMoved = append(this.objMoved, o)
}

func (up *user) GetPreviousPos() *TwoF {
	return &TwoF{up.prevCoord.X, up.prevCoord.Y}
}

// Compose a message to the client to update block change. The player must not be locked.
func (up *user) SendMessageBlockUpdate(cc chunkdb.CC, dx uint8, dy uint8, dz uint8, blType block) {
	const length = 19
	var ans [length]byte
	ans[0] = byte(length & 0xFF)
	ans[1] = byte(length >> 8)
	ans[2] = client_prot.CMD_BLOCK_UPDATE
	EncodeUint32(uint32(cc.X), ans[3:7])
	EncodeUint32(uint32(cc.Y), ans[7:11])
	EncodeUint32(uint32(cc.Z), ans[11:15])
	ans[15], ans[16], ans[17] = dx, dy, dz
	ans[18] = uint8(blType)
	up.writeNonBlocking(ans[:])
}

// Report the inventory for one item to a player.
// The amount can be 0. The purpose of this function is to update the client for a specific
// inventory item.
func ReportOneInventoryItem_WluBl(up *user, code string, lvl uint32) {
	const msgLen = 12
	b := make([]byte, msgLen)
	b[0] = byte(msgLen)
	// b[1] = 0
	b[2] = client_prot.CMD_UPD_INV
	for j := 0; j < 4; j++ {
		b[3+j] = code[j]
	}
	// b[7] = 0 // Default count is 0
	EncodeUint32(lvl, b[8:12])
	up.RLock()
	inv := up.pl.inventory
	l := inv.Len()
	for i := 0; i < l; i++ {
		obj := inv.Get(i)
		if obj.ID() == code && (lvl == 0 || lvl == obj.GetLevel()) {
			count := obj.GetCount()
			if count > math.MaxUint8 {
				count = math.MaxUint8 // This is what can be shown to the client
			}
			b[7] = count
			break
		}
	}
	up.RUnlock()
	// Wait with the actual writing until after unlocking the player.
	up.writeNonBlocking(b)
	// log.Println(b)
}

func AddOneObjectToUser_WLuBl(up *user, obj inventory.Object) {
	up.Lock()
	up.pl.inventory.Add(obj)
	up.Unlock()
	ReportOneInventoryItem_WluBl(up, obj.ID(), obj.GetLevel())
}
