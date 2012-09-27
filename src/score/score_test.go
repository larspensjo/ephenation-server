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

package score

import (
	"testing"
	"time"
)

const (
	uid              = 0
	initScore        = 1 // Seed
	initScoreBalance = 2 // Seed
)

func TestScoreInitial(t *testing.T) {
	// Close the process, as we do not want to update the SQL DB.
	if !Close() {
		t.FailNow()
	}
	sc, scb := Get(uid)
	if sc != 0 || scb != ConfigScoreBalanceZero {
		t.Errorf("Bad initial values score %v and scoreBalance %v\n", sc, scb)
	}

	now := time.Now()
	Initialize(uid, initScore, initScoreBalance, uint32(now.Unix()), "test", 1)
	sc, scb = Get(uid)
	// Some decay can be expected if time enough has passed. Score (sc) should decay downwards,
	// and ScoreBalance (scb) upwards.
	if !almost(sc, initScore) || !almost(initScoreBalance, scb) {
		t.Errorf("Failed to initialize correctly (score %v, scoreBalance %v)\n", sc, scb)
	}

	ts := getTerritoryScore(uid)
	delay := now.Add(time.Millisecond * 10)
	ts.decay(&delay)
	sc, scb = Get(uid)
	if !almost(sc, initScore) || !almost(initScoreBalance, scb) {
		t.Errorf("Failed to initialize correctly (score %v, scoreBalance %v)\n", sc, scb)
	}

	Add(uid, 1)
	sc, scb = Get(uid)
	if !almost(sc, initScore+1) || !almost(initScoreBalance+1, scb) {
		t.Errorf("Incorrect add, leading to %v, %v\n", sc, scb)
		t.FailNow()
	}

	// There is enough points for one payment
	if !Pay(uid, initScoreBalance+1) {
		t.Error("First pay failed")
		t.FailNow()
	}

	// But not for two
	sc, scb = Get(uid)
	if Pay(uid, 1) {
		t.Errorf("Second pay succeeded, was %v\n", scb)
	}
}

// Return true if 'a' is within 1% below 'b'.
func almost(a, b float64) bool {
	diff := (b - a) / b
	return diff >= 0 && diff <= 0.1
}
