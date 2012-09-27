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

package traffic

import (
	"fmt"
	"time"
)

//
// This package will simply keep track of amount of data received and sent,
// as well as provide statistics.
//

func New() *stat {
	ret := new(stat)
	go ret.computeAverage()
	return ret
}

type stat struct {
	totalSent, totalReceived int64
	avgSent, avgRec          float32
}

func (this *stat) AddSend(amount int) {
	this.totalSent += int64(amount)
}

func (this *stat) AddReceived(amount int) {
	this.totalReceived += int64(amount)
}

func (this *stat) String() string {
	return fmt.Sprintf("Received: %.2f MB (avg %d/s), Sent: %.2f MB (avg %d/s)", float64(this.totalReceived)/1e6, int(this.avgRec), float64(this.totalSent)/1e6, int(this.avgSent))
}

func (this *stat) computeAverage() {
	const sleepTime = 3e10
	for {
		prevSent := this.totalSent
		prevRec := this.totalReceived
		time.Sleep(sleepTime)
		diffSent := this.totalSent - prevSent
		diffRec := this.totalReceived - prevRec
		decay := float32(0.1) // TODO: Should use a decay computed correctly from the sleepTime
		this.avgSent = this.avgSent*decay + float32(diffSent)/sleepTime*1e9*(1-decay)
		this.avgRec = this.avgRec*decay + float32(diffRec)/sleepTime*1e9*(1-decay)
	}
}
