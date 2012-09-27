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
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"time"
)

//
// Simulate a player, setting up a process that will generate
// commands now and then.
func SimulatePlayer(ch chan msg_command, conn net.Conn) {
	moving := false
	request_coord(conn)
	for i := 0; ; i++ {
		time.Sleep(100000000) // Delay 100 ms
		rnd := rand.Uint32()
		switch rnd % 10 {
		case 0:
			request_chunk(conn) // Request a chunk
		case 1:
			moving = !moving
			if moving {
				SendMsg(conn, []byte{3, 0, client_prot.CMD_START_FWD})
			} else {
				SendMsg(conn, []byte{3, 0, client_prot.CMD_STOP_FWD})
			}
		case 2:
			dir := uint16(float32(rand.Uint32()%360) / 360 * 2 * math.Pi * 1000)
			h0 := uint8(dir & 0xff)
			h1 := uint8(dir >> 8)
			SendMsg(conn, []byte{7, 0, client_prot.CMD_SET_DIR, h0, h1, 0, 0})
		}
	}
}

// Send a command to request the coordinates
func request_coord(conn net.Conn) {
	const length = 3
	var ans [length]byte
	ans[0] = byte(length & 0xFF)
	ans[1] = byte(length >> 8)
	ans[2] = client_prot.CMD_GET_COORDINATE
	SendMsg(conn, ans[:])
}

func request_chunk(conn net.Conn) {
	const length = 3 + 3*4
	var ans [length]byte
	ans[0] = byte(length & 0xFF)
	ans[1] = byte(length >> 8)
	ans[2] = client_prot.CMD_READ_CHUNK
	x := uint32(xpos / 3200)
	for i := uint(0); i < 4; i++ {
		ans[i+3] = byte((x >> (i * 8)) & 0xFF)
	}
	y := uint32(ypos / 3200)
	for i := uint(0); i < 4; i++ {
		ans[i+3+4] = byte((y >> (i * 8)) & 0xFF)
	}
	z := uint32(zpos / 3200)
	for i := uint(0); i < 4; i++ {
		ans[i+3+8] = byte((z >> (i * 8)) & 0xFF)
	}
	SendMsg(conn, ans[:])
}

func SendMsg(conn net.Conn, b []byte) {
	if *vFlag > 1 {
		fmt.Printf("SendMsg: %v\n", b)
	}
	n, err := conn.Write(b)
	if err != nil {
		fmt.Printf("Failed to write %d bytes to connection: %s\n", len(b), err)
		os.Exit(1)
	}
	if n != len(b) {
		fmt.Printf("Could only write %d bytes out of %d\n", n, len(b))
		os.Exit(1)
	}
}
