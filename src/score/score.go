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

//
// This package will keep the score of territories.
//

import (
	"ephenationdb"
	"flag"
	"fmt"
	"io"
	"launchpad.net/tomb"
	"log"
	"math"
	"mysql"
	"sync"
	"time"
	"timerstats"
)

const (
	ConfigScoreUpdatePeriod = 1e11                // The perdiod between updates of teh SQL DB
	ConfigScoreHalfLife     = time.Hour * 24 * 30 // The half life off the score decay
	ConfigScoreBalHalfLife  = time.Hour * 24 * 5  // The half life off the scoreBalance decay
	ConfigScoreBalanceZero  = 10                  // The value that ScoreBalance will decay to, which is not 0
	ConfigHandicapLimit     = 10                  // Any more chunks than this will impose a handicap on the score
)

func init() {
	go maintainScore()
}

type territoryScore struct {
	Score        float64   // Number of seconds players spent in this territory
	ScoreBalance float64   // The total score with the payment subtracted
	handicap     float64   // How much to scale Score with. This is 1 for most players.
	TimeStamp    time.Time // The score will decay depending on this time
	uid          uint32    // Owner
	initialized  bool      // If the data has been initialized from the SQL DB
	modified     bool      // Has the value changed since last it was saved?
	name         string    // Not really needed, but nice for info
}

var (
	scores      = make(map[uint32]*territoryScore) // This map must always be locked by a mutex.
	procStatus  tomb.Tomb                          // Used to monitor the state of the process
	mutex       sync.RWMutex
	chScoreList = make(chan *territoryScore, 100)
	disableSQL  = flag.Bool("score.disableSQL", false, "Disable all updates with the SQL DB")
)

// Helper function. Get a pointer to the score for the specified 'uid'.
// The value pointed at can change asynchronously, after the lock is removed.
func getTerritoryScore(uid uint32) *territoryScore {
	// Need a read lock on the map
	mutex.RLock()
	ts := scores[uid]
	mutex.RUnlock()
	if ts == nil {
		// There was no data yet for 'uid'. Now a write lock is needed, but it is not the usual case.
		ts = new(territoryScore)
		ts.TimeStamp = time.Now() // Initialize time
		ts.ScoreBalance = ConfigScoreBalanceZero
		ts.handicap = 1
		ts.uid = uid
		mutex.Lock()
		scores[uid] = ts
		mutex.Unlock()
		// Also send a copy to the maintenance process
		chScoreList <- ts
	}
	return ts
}

// Add points to the Score and BalanceScore.
func Add(uid uint32, points float64) {
	ts := getTerritoryScore(uid)
	ts.Score += points * ts.handicap
	ts.ScoreBalance += points
	ts.modified = true
	// log.Println("score.Add", uid, points, fact, ts)
}

// Given the number of chunks for a player, compute a factor used to decrease the player score with
func computeFactor(numChunks int) float64 {
	terr := float64(numChunks)
	// Any number less than ConfigHandicapLimit shall count as ConfigHandicapLimit. That means that it will only be an effective decrease for
	// players with more than ConfigHandicapLimit chunks.
	if terr < ConfigHandicapLimit {
		terr = ConfigHandicapLimit
	}
	return ConfigHandicapLimit / terr
}

// There is a report, usually from a player logging in, of the score for a player. Use it if we had no value yet.
// That way, there will be no need to later initialize the value from the SQL DB. The number of chunks is used to
// compute a handicap for players with more chunks than standard.
func Initialize(uid uint32, points, scoreBalance float64, timestamp uint32, name string, numChunks int) {
	ts := getTerritoryScore(uid)
	if !ts.initialized {
		// TODO: Use an atomic test-modify-write here for ts.initialized
		// Add the value from the SQL DB to the current values
		ts.initialized = true
		oldHandicap := ts.handicap
		ts.handicap = computeFactor(numChunks)
		ts.Score = ts.Score*ts.handicap/oldHandicap + points
		ts.ScoreBalance += scoreBalance - ConfigScoreBalanceZero // Deduct the initial ConfigScoreBalanceZero value
		ts.TimeStamp = time.Unix(int64(timestamp), 0)
		ts.name = name
		ts.modified = true
		now := time.Now()
		ts.decay(&now) // Update the decay
		log.Printf("score.Initialize after decay to %.2f balance %.2f, handicap %.2f, from %.2f, %.2f, %.2f",
			ts.Score, ts.ScoreBalance, ts.handicap, points, scoreBalance, float64(timestamp)/3600)
	}
}

// Pay 'cost' for a reward, and return true if the ScoreBalance was enough.
func Pay(uid uint32, cost float64) bool {
	ts := getTerritoryScore(uid)
	// As there is no lock, there is a small chance that another call is executed at the same time.
	// If this happens, the worst case is that too much payment is accepted. This is unlikely and not fatal
	if ts.ScoreBalance < cost {
		return false
	}
	ts.ScoreBalance -= cost
	ts.modified = true
	return true
}

// Get the score and the score balance. This is a debug function, and should normally not be needed.
func Get(uid uint32) (float64, float64) {
	ts := getTerritoryScore(uid)
	return ts.Score, ts.ScoreBalance
}

// Close the update process, and return the status
func Close() (ret bool) {
	// log.Println("score.Close initiating")
	procStatus.Kill(nil)

	// Wait for completion
	ret = true
	if err := procStatus.Wait(); err != nil {
		log.Println(err)
		ret = false
	}
	// log.Println("score.Close done")
	return
}

func Report(wr io.Writer) {
	now := time.Now()
	mutex.RLock()
	for uid, ts := range scores {
		age := float64(now.Sub(ts.TimeStamp)) / float64(time.Hour)
		fmt.Fprintf(wr, "%s (Uid %d) Score %.1f Balance %.1f Handicap %.1f (age %.2f hours)", ts.name, uid, ts.Score, ts.ScoreBalance, ts.handicap, age)
	}
	mutex.RUnlock()
}

// Given the time stamp, decay the scores
func (ts *territoryScore) decay(now *time.Time) {
	// The decay is based on an exponential half time
	deltaTime := float64(now.Sub(ts.TimeStamp))
	ts.TimeStamp = *now
	ts.modified = true

	// Update decay of Score
	ts.Score *= math.Exp2(-deltaTime / float64(ConfigScoreHalfLife))

	// Update the decay of ScoreBalance. Subtract the offset before doing the decay, and add it
	// back again afterwards.
	bal := ts.ScoreBalance - ConfigScoreBalanceZero
	ts.ScoreBalance = bal*math.Exp2(-deltaTime/float64(ConfigScoreBalHalfLife)) + ConfigScoreBalanceZero
}

//
// The rest of this file is for the process that maintains the score synchronisation with the SQL DB
//
func maintainScore() {
	// log.Println("score.maintainScore starting")
	var elapsed time.Duration // Used to measure performance of this process
	timerstats.Add("Score maintenance", ConfigScoreUpdatePeriod, &elapsed)
	var list []*territoryScore // Use a linear copy of all pointers so that the map doesn't have to be locked.
	defer procStatus.Done()
	// Stay in this loop until system shuts down (using the tomb control)
	for {
		start := time.Now()
		timer := time.NewTimer(ConfigScoreUpdatePeriod) // Used as the ticker to activate the process regularly
	again:
		select {
		case <-procStatus.Dying():
			update(list) // Save any remaining data not updated before terminating
			return
		case ts := <-chScoreList: // This is the way new entries are received
			list = append(list, ts)
			goto again // Don't start a new timer
		case <-timer.C:
			update(list)
			elapsed = time.Now().Sub(start)
		}
	}
}

// Iterate over the copy of all scores, and save to SQL DB where needed.
func update(list []*territoryScore) {
	if *disableSQL {
		return
	}
	db := ephenationdb.New()
	if db == nil {
		return // Give it up for this time
	}
	defer ephenationdb.Release(db)
	now := time.Now()
	for _, ts := range list {
		if !ts.initialized {
			log.Println("score.update initialize from DB for", ts.uid)
			loadFromSQL(db, ts)
		}

		if !ts.modified {
			continue
		}

		oldScore := ts.Score
		oldBalance := ts.ScoreBalance
		ts.decay(&now)

		if ts.initialized {
			// Normally, it is initialized, but stay on the safe side if there was a problem.
			saveToSQL(db, ts)
			log.Printf("score.update %s to %.1f (diff %f), Balance %.1f (diff %f)\n", ts.name, ts.Score, ts.Score-oldScore, ts.ScoreBalance, ts.ScoreBalance-oldBalance)
		}
	}
}

// Load SDB DB score data for territory ts.uid.
func loadFromSQL(db *mysql.Client, ts *territoryScore) {
	query := "SELECT TScoreTotal,TScoreBalance,TScoreTime,name FROM avatars WHERE ID='" + fmt.Sprint(ts.uid) + "'"
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Println(err)
		return
	}

	// Execute statement
	err = stmt.Execute()
	if err != nil {
		log.Println(err)
		return
	}

	var terrScore, terrScoreBalance float64
	var terrScoreTimestamp uint32
	var name string
	stmt.BindResult(&terrScore, &terrScoreBalance, &terrScoreTimestamp, &name)
	for {
		eof, err := stmt.Fetch()
		if err != nil {
			log.Println(err)
			return
		}
		if eof {
			break
		}
	}
	numChunks := countChunks(db, ts.uid)
	Initialize(ts.uid, terrScore, terrScoreBalance, terrScoreTimestamp, name, numChunks)
}

func saveToSQL(db *mysql.Client, ts *territoryScore) {
	ts.modified = false
	query := "UPDATE avatars SET TScoreTotal=?,TScoreBalance=?,TScoreTime=? WHERE ID='" + fmt.Sprint(ts.uid) + "'"

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Println(err)
		return
	}

	stmt.BindParams(ts.Score, ts.ScoreBalance, ts.TimeStamp.Unix())

	err = stmt.Execute()
	if err != nil {
		log.Println(err)
		return
	}
}

// Count the number of chunks this player has
func countChunks(db *mysql.Client, uid uint32) int {
	// Build a query for the given chunk coordinate as an argument
	query := fmt.Sprintf("SELECT * FROM chunkdata WHERE avatarID=%d", uid)
	err := db.Query(query)
	if err != nil {
		// Fatal error
		log.Println(err)
		return ConfigHandicapLimit
	}

	// Store the result
	result, err := db.StoreResult()
	if err != nil {
		log.Println(err)
		return ConfigHandicapLimit
	}
	numRows := result.RowCount()

	db.FreeResult()
	return int(numRows)
}
