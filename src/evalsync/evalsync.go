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

package evalsync

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	limitForFast = 1e5 // Nanoseconds
)

var (
	numFastRead        int
	numSlowRead        int
	numFastWrite       int
	numSlowWrite       int
	worstReadDelay     time.Duration
	worstWriteDelay    time.Duration
	worstReadOp        time.Duration
	worstWriteOp       time.Duration
	worstReadDelayMsg  string
	worstWriteDelayMsg string
	worstReadOpMsg     string
	worstWriteOpMsg    string
	evalsyncFlag       = flag.Int("evalsync.v", 0, "Enable mutex measurements")
)

type RWMutex struct {
	rwMutex    sync.RWMutex
	startWrite time.Time
	startRead  time.Time
	Ignore     bool // Ignore statistics on this mutex
	rlocked    bool // Used for statistics
	wlocked    bool // used for statistics
	file       string
	line       int
	savetrace  bool
}

// Return an array of strings with information
func Eval() []string {
	stats := fmt.Sprintf("Slow read %.0f%%, slow write %.0f%%",
		float32(numSlowRead)/float32(numFastRead+numSlowRead)*100,
		float32(numSlowWrite)/float32(numFastWrite+numSlowWrite)*100)
	numFastRead = 0
	numSlowRead = 0
	worstReadDelay = 0
	numFastWrite = 0
	numSlowWrite = 0
	worstWriteDelay = 0
	worstReadOp = 0
	worstWriteOp = 0
	return []string{stats, worstReadDelayMsg, worstWriteDelayMsg, worstReadOpMsg, worstWriteOpMsg}
}

func fname(path string) string {
	split := strings.Split(path, "/")
	last := len(split) - 1
	return split[last]
}

func (rw *RWMutex) Lock() {
	var tm time.Time
	wasBlocking := false
	if *evalsyncFlag > 0 {
		if rw.rlocked || rw.wlocked {
			// This mutex was already locked by someone else. Ask that process to save a trace.
			rw.savetrace = true
			wasBlocking = true
		}
		tm = time.Now()
		if *evalsyncFlag > 1 && rw.savetrace {
			_, file, line, _ := runtime.Caller(1)
			log.Printf("Lock req (from %v:%v). Already locked: %v\n", fname(file), line, rw.savetrace)
		}
	}
	rw.rwMutex.Lock()
	if *evalsyncFlag > 0 {
		if *evalsyncFlag > 1 && wasBlocking {
			_, file, line, _ := runtime.Caller(1)
			log.Printf("Lock ack (from %v:%v) saved %v\n", fname(file), line, rw.file)
		}
		rw.wlocked = true
		rw.startWrite = time.Now()
		diff := rw.startWrite.Sub(tm)
		if diff > limitForFast {
			numSlowWrite++
		} else {
			numFastWrite++
		}
		if diff > worstWriteDelay {
			worstWriteDelay = diff
			_, file, line, ok := runtime.Caller(1)
			if ok {
				sub := strings.Split(file, "/")
				worstWriteDelayMsg = fmt.Sprintf("WR del %.6fs: %s:%d", float32(diff)/1e9, sub[len(sub)-1], line)
				if rw.file != "" {
					// There is a trace from the locker before.
					sub = strings.Split(rw.file, "/")
					worstWriteDelayMsg += fmt.Sprintf(" (Before was %s:%d)", sub[len(sub)-1], rw.line)
					rw.file = ""
				}
			}
		}
	}
}

func (rw *RWMutex) Unlock() {
	if *evalsyncFlag > 0 && rw.savetrace {
		if *evalsyncFlag > 1 {
			_, file, line, _ := runtime.Caller(1)
			log.Printf("Unlock from %v:%v\n", fname(file), line)
		}
		diff := time.Now().Sub(rw.startWrite)
		if diff > worstWriteOp {
			worstWriteOp = diff
			_, file, line, ok := runtime.Caller(1)
			if ok {
				sub := strings.Split(file, "/")
				worstWriteOpMsg = fmt.Sprintf("WR op  %.6fs: %s:%d", float32(diff)/1e9, sub[len(sub)-1], line)
			}
		}
		rw.wlocked = false
		if rw.savetrace {
			// Another process asked this lock to save a trace.
			_, rw.file, rw.line, _ = runtime.Caller(1)
			rw.savetrace = false
		}
	}
	rw.rwMutex.Unlock()
}

func (rw *RWMutex) RLock() {
	var tm time.Time
	wasBlocking := false
	if *evalsyncFlag > 0 {
		if rw.wlocked {
			// This mutex was already locked by someone else. Ask that process to save a trace.
			rw.savetrace = true
			wasBlocking = true
		}
		tm = time.Now()
		if *evalsyncFlag > 1 && rw.savetrace {
			_, file, line, _ := runtime.Caller(1)
			log.Printf("RLock req (from %v:%v). Already locked: %v\n", fname(file), line, rw.savetrace)
		}
	}
	rw.rwMutex.RLock()
	if *evalsyncFlag > 0 {
		if *evalsyncFlag > 1 && wasBlocking {
			_, file, line, _ := runtime.Caller(1)
			log.Printf("RLock ack from %v:%v, saved %v\n", fname(file), line, rw.file)
		}
		rw.rlocked = true
		rw.startRead = time.Now()
		diff := rw.startRead.Sub(tm)
		// fmt.Print(diff, " ")
		if diff > limitForFast {
			numSlowRead++
		} else {
			numFastRead++
		}
		if diff > worstReadDelay {
			worstReadDelay = diff
			_, file, line, ok := runtime.Caller(1)
			if ok {
				sub := strings.Split(file, "/")
				worstReadDelayMsg = fmt.Sprintf("RD del %.6fs: %s:%d", float32(diff)/1e9, sub[len(sub)-1], line)
				if rw.file != "" {
					// There is a trace from the locker before.
					sub = strings.Split(rw.file, "/")
					worstWriteDelayMsg += fmt.Sprintf(" (Before was %s:%d)", sub[len(sub)-1], rw.line)
					rw.file = ""
				}
			}
		}
	}
}

func (rw *RWMutex) RUnlock() {
	if *evalsyncFlag > 0 {
		if *evalsyncFlag > 1 && rw.savetrace {
			_, file, line, _ := runtime.Caller(1)
			log.Printf("RUnlock from %v:%v\n", fname(file), line)
		}
		diff := time.Now().Sub(rw.startRead)
		if diff > worstReadOp {
			worstReadOp = diff
			_, file, line, ok := runtime.Caller(1)
			if ok {
				sub := strings.Split(file, "/")
				worstReadOpMsg = fmt.Sprintf("RD op  %.6fs: %s:%d", float32(diff)/1e9, sub[len(sub)-1], line)
			}
		}
		rw.rlocked = false
		if rw.savetrace {
			// Another process asked this lock to save a trace.
			_, rw.file, rw.line, _ = runtime.Caller(1)
			rw.savetrace = false
		}
	}
	rw.rwMutex.RUnlock()
}
