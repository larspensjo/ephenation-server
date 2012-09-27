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

package timerstats

//
// Keep tab of the list of timers, to make it easy to query for statistics. This will give a snapshot of
// the last timer, not an average.
//

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

type TimerStatistic struct {
	timer       *time.Duration
	sleepPeriod time.Duration
	descr       string
}

type collection []TimerStatistic

var timerStatSync sync.Mutex

var allTimerStatistics collection

func (ts TimerStatistic) fraction() float64 {
	if *ts.timer == 0 {
		// There was no reported timer value yet
		return 0
	}
	return float64(*ts.timer-ts.sleepPeriod) / float64(ts.sleepPeriod)
}

func Report(wr io.Writer) {
	timerStatSync.Lock()
	// fmt.Printf("Before sort: %#v\n", allTimerStatistics)
	sort.Sort(&allTimerStatistics)
	// fmt.Printf("After sort: %#v\n", allTimerStatistics)
	for _, stat := range allTimerStatistics {
		fmt.Fprintf(wr, "%8.4f (%5.2f per cent) %s\n", float64(*stat.timer)/1e9, stat.fraction()*100, stat.descr)
	}
	timerStatSync.Unlock()
	return
}

// Add a procedure.
// descr: A description to be used in the report
// sleepPeriod: The expected sleeping period of the procedure
// timer: Pointer to actual sleeping period, managed by the caller.
func Add(descr string, sleepPeriod time.Duration, timer *time.Duration) {
	timerStatSync.Lock()
	stat := TimerStatistic{timer, sleepPeriod, descr}
	allTimerStatistics = append(allTimerStatistics, stat)
	timerStatSync.Unlock()
	// fmt.Printf("AddTimerStatistic: %v\n", allTimerStatistics)
}

// Provide functions needed to fulfil the sorting interface
func (timers *collection) Len() int {
	return len(*timers)
}

func (timers *collection) Less(i, j int) bool {
	return (*timers)[i].fraction() < (*timers)[j].fraction()
}

func (timers *collection) Swap(i, j int) {
	slice := *timers
	tmr := slice[i]
	slice[i] = slice[j]
	slice[j] = tmr
}
