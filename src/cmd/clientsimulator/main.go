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
	"math"
	"math/rand"
	"net"
	"sync"
	"time"
)

var nFlag *int = flag.Int("n", 1, "Number of instances")
var uFlag *string = flag.String("u", "test", "Name prefix of players")
var aFlag *string = flag.String("a", "127.0.0.1", "The network address")
var vFlag *int = flag.Int("v", 0, "Verbose")
var pFlag *string = flag.String("p", "57862", "The network port")
var oFlag *int = flag.Int("o", 1, "Offset number on player name")

var waitForAck sync.Mutex

func main() {
	flag.Parse()
	var i int
	user := *uFlag
	num_instances := *nFlag
	ipaddr := *aFlag
	port := *pFlag
	addr := ipaddr + ":" + port

	rand.Seed(int64(time.Now().Nanosecond()))

	for i = 0; i < num_instances; i++ {
		go spawn_simulator(addr, user+fmt.Sprint(i+*oFlag))
	}

	fmt.Printf("Started %d instances\n", num_instances)

	time.Sleep(math.MaxInt64 / 2) // More or less for ever
}

func spawn_simulator(addr string, user string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Printf("Connection to %s failed: %v\n", addr, err)
		return
	}
	cmd := string(len(user)+3) + "\000"
	login_cmd := []byte(cmd + string(client_prot.CMD_LOGIN) + user)
	waitForAck.Lock() // Will be unlocked by the login acknowledge
	if *vFlag >= 2 {
		fmt.Printf("Login %v. ", user)
	}
	SendMsg(conn, login_cmd)
	CentralControl(conn, user)
}
