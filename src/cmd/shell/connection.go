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

func connect(addr string, user string) net.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Printf("Connection to %s failed: %v\n", addr, err)
		os.Exit(1)
	}
	cmd := string(len(user)+3) + "\000"
	login_cmd := []byte(cmd + string(client_prot.CMD_LOGIN) + user)
	// fmt.Printf("Login command: %v\n", login_cmd)
	SendMsg(conn, login_cmd)
	return conn
}
