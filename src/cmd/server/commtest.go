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
	"bytes"
	"client_prot"
	"fmt"
	"net"
	"runtime"
	"time"
)

type dummyConn struct {
	// Use a channel to feed the readers for this connection.
	ch       chan []byte // Send byte array slices
	slice    []byte      // Remainder of previous slice not read yet.
	cmdFlags [client_prot.CMD_Last]bool
	open     bool
	buffer   bytes.Buffer // Buffer written data until there is a complete message
}

type dummyAddr struct{}

func (dc *dummyConn) Close() error                  { dc.open = false; return nil }
func (*dummyConn) LocalAddr() net.Addr              { return dummyAddr{} }
func (*dummyConn) RemoteAddr() net.Addr             { return dummyAddr{} }
func (*dummyConn) SetTimeout(nsec int64) error      { return nil }
func (*dummyConn) SetReadTimeout(nsec int64) error  { return nil }
func (*dummyConn) SetWriteTimeout(nsec int64) error { return nil }

func (dc *dummyConn) Read(b []byte) (n int, err error) {
	_, file, line, ok := runtime.Caller(1)
	if ok {
		fmt.Printf("dummyConn Read: called from %s:%d\n", file, line)
	}
	if dc.slice == nil {
		dc.slice = <-dc.ch
	}
	n = copy(b, dc.slice)
	if n == len(dc.slice) {
		dc.slice = nil
	} else {
		dc.slice = dc.slice[n:]
	}
	if ok {
		fmt.Printf("dummyConn Read: returning %v to %s:%d\n", b, file, line)
	}
	return
}

// Inject a byte stream, to be used by the process on Read(). Not currently
// used, as it is tricky to test with parallel processes.
func (dc *dummyConn) Inject(b []byte) {
	dc.ch <- b
}

// Don't use the channel, we want to look at the data that
// was going to be written.
func (dc *dummyConn) Write(b []byte) (n int, err error) {
	// Notice that the message may not be complete.
	dc.buffer.Write(b)
	buff := dc.buffer.Bytes()
	// fmt.Printf("dummyConn:Write write buffer is now %v\n", buff)
	length := int(buff[0]) + int(buff[1])<<8
	if length > len(buff) {
		// Waiting for more data
		return len(b), nil
	}
	dc.cmdFlags[buff[2]] = true // Remember that this command has been seen.
	switch buff[2] {
	case client_prot.CMD_MESSAGE: // Ignore
	case client_prot.CMD_LOGIN_ACK: // Ignore
	case client_prot.CMD_OBJECT_LIST: // Ignore
	case client_prot.CMD_REQ_PASSWORD: // Ignore
	case client_prot.CMD_REPORT_COORDINATE: // Ignore
	case client_prot.CMD_RESP_PLAYER_HIT_MONSTER: // Ignore
	case client_prot.CMD_EQUIPMENT: // Ignore
	default:
		fmt.Printf("dummyConn:Write unexpected %d: %v\n", len(buff), buff)
	}
	dc.buffer.Reset()
	return len(b), nil
}

// Test if a command has been seen, and clear the flag afterwards.
func (dc *dummyConn) TestCommandSeen(i int) (ret bool) {
	ret = dc.cmdFlags[i]
	dc.cmdFlags[i] = false
	return
}

func (dc *dummyConn) TestOpen() bool {
	return dc.open
}

func (dc *dummyConn) SetReadDeadline(t time.Time) error  { return nil }
func (dc *dummyConn) SetDeadline(t time.Time) error      { return nil }
func (dc *dummyConn) SetWriteDeadline(t time.Time) error { return nil }

func MakeDummyConn() *dummyConn {
	return &dummyConn{ch: make(chan []byte), open: true}
}

func (dummyAddr) Network() string { return "127.0.0.1:1234" }
func (dummyAddr) String() string  { return "127.0.0.1:1234" }
