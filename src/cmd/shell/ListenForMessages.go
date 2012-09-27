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
	"net"
	"os"
)

func ListenForServerMessages(conn net.Conn, user string) {
	var buff [10000]byte
	for {
		n, err := conn.Read(buff[0:2]) // Only the length
		if err != nil || n == 0 {
			if e2, ok := err.(*net.OpError); ok && e2.Temporary() {
				fmt.Println("ListenForServerMessages: temporary")
				continue
			}
			if e2, ok := err.(*net.OpError); ok && e2.Timeout() {
				fmt.Println("ListenForServerMessages: Timeout")
				continue
			}
			fmt.Printf("ListenForServerMessages: Failed to read: %v\n", err)
			os.Exit(1) // Major failure
		}
		if n == 1 {
			fmt.Printf("ListenForServerMessages: Got %d bytes reading from socket\n", n)
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
		case client_prot.CMD_OBJECT_LIST:
			if *vFlag >= 2 {
				fmt.Println("CMD_OBJECT_LIST")
			}
		case client_prot.CMD_MESSAGE:
			fmt.Printf("CMD_MESSAGE: %s\n", string(buff[3:length]))
		case client_prot.CMD_REPORT_COORDINATE:
			fmt.Println("CMD_REPORT_COORDINATE")
			fmt.Printf("ListenForServerMessages (l %d) cmd %v\n", length, buff[:n])
		case client_prot.CMD_CHUNK_ANSWER:
			fmt.Printf("CMD_CHUNK_ANSWER: Got chunk buffwer length %d: %v\n", length, buff[0:length])
		case client_prot.CMD_LOGIN_ACK:
			fmt.Println("User", user, "login ack")
		case client_prot.CMD_BLOCK_UPDATE:
			fmt.Println("CMD_BLOCK_UPDATE")
		case client_prot.CMD_REQ_PASSWORD:
			fmt.Println("CMD_REQ_PASSWORD")
		case client_prot.CMD_PROT_VERSION:
			fmt.Printf("Protocol version %d.%d\n", buff[5]+buff[6]<<8, buff[3]+buff[4]<<8)
		case client_prot.CMD_EQUIPMENT: // Ignore
		case client_prot.CMD_PLAYER_STATS: // Ignore
		case client_prot.CMD_RESP_AGGRO_FROM_MONSTER:
		default:
			fmt.Printf("Unknown command %v\n", buff[0:length])
		}
	}
}
