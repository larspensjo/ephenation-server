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
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// This command is used for testing performance of various constructs.

func main() {
	// Empty loop
	const EL = 1e9
	start := time.Now()
	for i := int64(0); i < EL; i++ {
	}
	elapsed := time.Now().Sub(start)
	if elapsed < 1e8 {
		fmt.Println("Too quick for empty loop")
	}
	fmt.Printf("Empty loop: %.3f ns\n", float32(elapsed)/EL)

	// Read unit32
	const RUINT = 1e9
	var v uint32
	var x uint32
	start = time.Now()
	for i := int64(0); i < RUINT; i++ {
		x = v
	}
	elapsed = time.Now().Sub(start)
	if elapsed < 1e8 {
		fmt.Println("Too quick for read unit32")
	}
	fmt.Printf("Read uint32: %.3f ns\n", float32(elapsed)/RUINT)

	// Measure time using mutex
	const ML = 1e7
	var s sync.RWMutex
	start = time.Now()
	for i := int64(0); i < ML; i++ {
		s.Lock()
		s.Unlock()
	}
	elapsed = time.Now().Sub(start)
	if elapsed < 1e8 {
		fmt.Println("Too quick mutex lock/unlock")
	}
	fmt.Printf("Mutex lock/unlock: %.3f ns\n", float32(elapsed)/ML)

	// Measure time using read lock mutex
	const RL = 1e7
	start = time.Now()
	for i := int64(0); i < RL; i++ {
		s.RLock()
		s.RUnlock()
	}
	elapsed = time.Now().Sub(start)
	if elapsed < 1e8 {
		fmt.Println("Too quick for mutex rlock/runlock")
	}
	fmt.Printf("Mutex RLock/RUnlock: %.3f ns\n", float32(elapsed)/RL)

	// Measure time using LoadUint32
	const LUINT = 1e8
	start = time.Now()
	for i := int64(0); i < LUINT; i++ {
		x = atomic.LoadUint32(&v)
	}
	x = x + 0 // Have to use 'x'.
	elapsed = time.Now().Sub(start)
	if elapsed < 1e8 {
		fmt.Println("Too quick for atomc.LoadUint32")
	}
	fmt.Printf("atomc.LoadUint32: %.3f ns\n", float32(elapsed)/LUINT)

	// Measure time using channel
	const CT = 1e7
	ch := make(chan int, 1)
	start = time.Now()
	for i := int64(0); i < CT; i++ {
		ch <- 0
		<-ch
	}
	elapsed = time.Now().Sub(start)
	if elapsed < 1e8 {
		fmt.Println("Too quick for Write/read channel")
	}
	fmt.Printf("Write/read channel: %.3f ns\n", float32(elapsed)/CT)

	tst := uint64(0x1234567812345678)
	runtime.GOMAXPROCS(4)
	go Reader(ch, &tst)
	go Writer(ch, &tst)
	go Writer(ch, &tst)
	<-ch
	<-ch
	<-ch
}

func Writer(ch chan int, p *uint64) {
	fmt.Println("Writer starting")
	for i := int64(0); i < 5e9; i++ {
		*p = 0x1234567812345678
		*p = 0x8765432187654321
		*p = 0x1234567812345678
		*p = 0x8765432187654321
		*p = 0x1234567812345678
		*p = 0x8765432187654321
		*p = 0x1234567812345678
		*p = 0x8765432187654321
		*p = 0x1234567812345678
		*p = 0x8765432187654321
	}
	fmt.Println("Writer done")
	ch <- 0
}

func Reader(ch chan int, p *uint64) {
	fmt.Println("Reader starting")
	var v uint64
	for i := int64(0); i < 2e9; i++ {
		v = *p
		if v != 0x1234567812345678 && v != 0x8765432187654321 {
			fmt.Printf("Reader: got %x\n", v)
		}
		v = *p
		if v != 0x1234567812345678 && v != 0x8765432187654321 {
			fmt.Printf("Reader: got %x\n", v)
		}
	}
	fmt.Println("Reader done")
	ch <- 1
}
