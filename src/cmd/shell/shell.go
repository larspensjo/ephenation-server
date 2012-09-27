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
	"flag"
	"fmt"
	"os"
)

var uFlag *string = flag.String("u", "test0", "Name prefix of players")
var aFlag *string = flag.String("a", "127.0.0.1:57862", "The network address")
var vFlag *int = flag.Int("v", 0, "Verbose")

func main() {
	flag.Parse()
	user := *uFlag
	addr := *aFlag

	conn := connect(addr, user)
	go ListenForServerMessages(conn, user)

	b := make([]byte, 1000)
	for {
		n, err := os.Stdin.Read(b)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if n == 1 {
			continue // Empoty line, only trailing LF
		}
		if b[0] == '/' {
			SendMsg(conn, []byte{byte(n + 2), 0, client_prot.CMD_DEBUG})
			SendMsg(conn, b[:n-1]) // Skip the trailing newline
			continue
		}
	}
}
