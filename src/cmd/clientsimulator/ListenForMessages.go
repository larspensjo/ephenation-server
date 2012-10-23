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
// There are three processes created:
// 1. Simulating a player and generating messages that will be sent to the game server.
// 2. Listening for messsages being sent back from the game server.
// 3. The main control process, keeping track of the state.

package main

import (
	"client_prot"
	"fmt"
	"net"
	"os"
)

// All commands sent here has to match this interface.
type msg_command interface {
	Execute()
}

var xpos, ypos, zpos int64

func CentralControl(conn net.Conn, user string) {
	// Setup a channel callback messages.
	ch := make(chan msg_command)

	go SimulatePlayer(ch, conn) // Start the player simulator
	go ListenForServerMessages(ch, conn, user)

	for {
		cmd := <-ch
		cmd.Execute()
	}
}

func ListenForServerMessages(ch chan msg_command, conn net.Conn, user string) {
	var buff [15000]byte // TODO: Ugly naked constant
	for {
		// TODO: The mechanism for reading a message is ugly. Use something similar as the C++ client instead.
		n, err := conn.Read(buff[0:2]) // Only the length
		if err != nil || n == 0 {
			if e2, ok := err.(*net.OpError); ok && e2.Temporary() {
				fmt.Println("ListenForServerMessages: temporary")
				continue
			}
			if e2, ok := err.(*net.OpError); ok && e2.Timeout() {
				fmt.Println("ListenForServerMessages: timeout")
				continue
			}
			fmt.Printf("ListenForServerMessages: Failed to read: %v\n", err)
			os.Exit(1) // Major failure
		}
		if n == 1 {
			if *vFlag >= 1 {
				fmt.Printf("ListenForServerMessages: Got %d bytes reading from socket\n", n)
			}
			n2, err := conn.Read(buff[1:2]) // Second byte of the length
			if err != nil || n2 != 1 {
				fmt.Printf("ListenForServerMessages: Failed to read: %v\n", err)
				os.Exit(1) // Major failure
			}
			n = 2
		}
		length := int(uint(buff[1])<<8 + uint(buff[0]))
		if length < 3 || length > cap(buff) {
			fmt.Printf("ListenForServerMessages: User %v Expecting %d bytes, which is bad\n", user, length)
			conn.Read(buff[2:2])
			fmt.Println("Command was", buff[2])
			os.Exit(1)
		}
		n, err = conn.Read(buff[2:length]) // Read the rest.readin from socket
		if err != nil {
			fmt.Printf("ListenForServerMessages: Failed to read: %v\n", err)
			os.Exit(1) // Major failure
		}
		for tot := n + 2; tot < length; tot += n {
			n, err = conn.Read(buff[tot:length]) // Read the rest.
			if err != nil {
				fmt.Printf("ListenForServerMessages: Failed to read: %v\n", err)
				os.Exit(1) // Major failure
			}
			// fmt.Printf("Got trailing %d+%d bytes of %d\n", tot, n, length)
		}
		if *vFlag > 1 {
			fmt.Printf("ListenForServerMessages user %v Receive %v... (length %d)\n", user, buff[0:3], length)
		}
		// fmt.Printf("ListenForServerMessages (l %d) cmd %v\n", length, buff[:n])
		switch buff[2] {
		case client_prot.CMD_RESP_PLAYER_HIT_BY_MONSTER:
			if *vFlag > 1 {
				fmt.Printf("Player %v hit with %.0f\n", user, float32(buff[7])/255*100)
			}
		case client_prot.CMD_OBJECT_LIST:
			// List of players or other things. For now, ignore this.
		case client_prot.CMD_MESSAGE:
			if *vFlag > 1 {
				fmt.Printf("%s\n", string(buff[3:length]))
			}
		case client_prot.CMD_REPORT_COORDINATE:
			ch <- ReportCoordinateCommand{buff[3:length]}
		case client_prot.CMD_CHUNK_ANSWER:
			// fmt.Printf("Got chunk buffer length %d: %v\n", length, buff[0:length])
			ch <- ReportChunkCommand{buff[3:length]}
		case client_prot.CMD_LOGIN_ACK:
			if *vFlag > 0 {
				fmt.Println("User", user, "login ack")
			}
			waitForAck.Unlock()
		case client_prot.CMD_PLAYER_STATS:
			hp := buff[3]
			// exp := buff[4]
			// mana := buff[13]
			if hp == 0 {
				if *vFlag > 0 {
					fmt.Println(user, "dies.")
				}
				const length = 10
				var b [length]byte
				b[0] = length
				b[2] = client_prot.CMD_DEBUG
				msg := append(b[0:3], "/revive"...)
				SendMsg(conn, msg)
			}
		case client_prot.CMD_BLOCK_UPDATE: // For now, ignore this.
		case client_prot.CMD_REQ_PASSWORD: // Ignore
		case client_prot.CMD_EQUIPMENT:
		case client_prot.CMD_PROT_VERSION:
		case client_prot.CMD_RESP_PLAYER_HIT_MONSTER:
		case client_prot.CMD_JELLY_BLOCKS:
		case client_prot.CMD_RESP_AGGRO_FROM_MONSTER:
			monsterID := ParseUint32(buff[3:7])
			if *vFlag > 0 {
				fmt.Println("Aggro by monster", monsterID)
			}
			const length = 7
			var b [length]byte
			b[0] = length
			b[2] = client_prot.CMD_ATTACK_MONSTER
			for i := uint(0); i < 4; i++ {
				b[i+3] = byte((monsterID >> (i * 8)) & 0xFF)
			}
			SendMsg(conn, b[:])
		case client_prot.CMD_UPD_INV:
			fmt.Println(user, "got a drop")
		default:
			fmt.Printf("Unknown command %v\n", buff[0:length])
		}
	}
}

type ReportCoordinateCommand struct {
	data []byte // 3 x 8 bytes for the coordinates
}

func (rcc ReportCoordinateCommand) Execute() {
	var x uint64
	for i := uint(0); i < 8; i++ {
		x += uint64(rcc.data[i]) << (i * 8)
	}
	var y uint64
	for i := uint(0); i < 8; i++ {
		y += uint64(rcc.data[i+8]) << (i * 8)
	}
	var z uint64
	for i := uint(0); i < 8; i++ {
		z += uint64(rcc.data[i+16]) << (i * 8)
	}
	// fmt.Printf("ReportCoordinate: %v,%v,%v\n", float(x)/100, float(y)/100, float(z)/100)
	// fmt.Printf("ReportCoordinate %v\n", rcc.data)
	xpos, ypos, zpos = int64(x), int64(y), int64(z)
}

type ReportChunkCommand struct {
	data []byte // variable
}

func (rcc ReportChunkCommand) Execute() {
	// fmt.Printf("Got chunk length %d, %v\n", len(rcc.data), rcc.data)
	// For now, the chunk isn't used.
	// fmt.Printf("Got chunk %v\n", rcc.data)
}

func ParseUint32(b []byte) uint32 {
	if len(b) < 4 {
		panic("Wrong size")
	}
	var res uint32
	for i := 0; i < 4; i++ {
		res |= uint32(b[i]) << (uint(i) * 8)
	}
	return res
}
