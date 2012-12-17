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

//
// This package will keep the score of territories.
//
// There are two parallel tables of scores, pointing to the same data. One
// map is used for efficiently mapping from uid to a pointer to the score data,
// and one linear table is used to traverse the complete list for updates and saves.
// The reason for this is that there will be a lot of updates of scores, which has to be
// very efficient. Every such access have to lock the map. Whenever the whole list is
// traversed periodically, the map must not be locked as it takes some time.
//
// Note that the score entry itself is not locked. That means that there is a small chance
// that data can be corrupted. The worst case is a failed update, which is acceptable.
//
// The "BalanceScore" is similar to the total score, but it will decay towards a value greater than 0.
//
package score

import (
	"chunkdb"
	"ephenationdb"
	"flag"
	"fmt"
	"io"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"launchpad.net/tomb"
	"log"
	"math"
	"sync"
	"time"
	"timerstats"
)

const (
	ConfigScoreUpdatePeriod = 1e11                // The perdiod between updates of the database
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
	modified     bool      // Has the value changed since last it was saved?
	name         string    // Not really needed, but nice for info
}

var (
	scores      = make(map[uint32]*territoryScore) // This map must always be locked by a mutex.
	procStatus  tomb.Tomb                          // Used to monitor the state of the process
	mutex       sync.RWMutex                       // Used to protect the map
	chScoreList = make(chan *territoryScore, 100)
	disableSave = flag.Bool("score.DisableSave", false, "Disable all updates with the database")
)

// Helper function. Get a pointer to the score for the specified 'uid'.
func getTerritoryScore(uid uint32) *territoryScore {
	mutex.RLock()
	ts := scores[uid] // Need a read lock on the map
	mutex.RUnlock()
	if ts != nil {
		return ts
	}

	// There was no data yet for 'uid'.
	ts = new(territoryScore)
	loadFromSQL(ts, uid)
	mutex.Lock()
	scores[uid] = ts // Use a write lock
	mutex.Unlock()
	// Also send a copy to the maintenance process
	chScoreList <- ts
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

// Trig loading of a player
func Initialize(uid uint32) {
	getTerritoryScore(uid)
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

// Debug function
func Report(wr io.Writer) {
	now := time.Now()
	mutex.RLock()
	for uid, ts := range scores {
		age := float64(now.Sub(ts.TimeStamp)) / float64(time.Hour)
		fmt.Fprintf(wr, "%s (%d) Score %.1f Bal %.1f Hand %.1f (age %.2f hours)", ts.name, uid, ts.Score, ts.ScoreBalance, ts.handicap, age)
		if ts.Score > 0.1 {
			ts.decay(&now) // Trig a save if there is some score
		}
	}
	mutex.RUnlock()
}

// Given the time stamp, decay the score
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
// A process that will manage the decay of all entries, and save to the database as needed.
//
func maintainScore() {
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
			elapsed = time.Now().Sub(start) // For statistics only
		}
	}
}

// Iterate over the copy of the list of all scores, and save to database where needed.
// The map could be used here, in which case a lock would be required. But DB access take
// a long time.
func update(list []*territoryScore) {
	db := ephenationdb.New()
	now := time.Now()
	for _, ts := range list {
		if !ts.modified {
			continue
		}

		// The decay isn't executed unless there has been a change, to save from unnecessary DB updates.
		ts.decay(&now)
		saveToSQL(db, ts)
	}
}

// Load DB score data for territory owned by 'uid' into 'ts'.
func loadFromSQL(ts *territoryScore, uid uint32) {
	var avatarScore struct {
		TScoreTotal, TScoreBalance float64
		TScoreTime                 uint32
		Name                       string
		Territory                  []chunkdb.CC // The chunks allocated for this player.
	}
	db := ephenationdb.New()
	query := db.C("avatars").FindId(uid)
	err := query.One(&avatarScore)
	if err != nil {
		log.Println(err)
		return
	}

	ts.uid = uid
	ts.handicap = computeFactor(len(avatarScore.Territory))
	ts.Score = avatarScore.TScoreTotal
	ts.ScoreBalance = avatarScore.TScoreBalance
	ts.TimeStamp = time.Unix(int64(avatarScore.TScoreTime), 0)
	ts.name = avatarScore.Name
	ts.modified = true
	now := time.Now()
	ts.decay(&now) // Update the decay
}

func saveToSQL(db *mgo.Database, ts *territoryScore) {
	if *disableSave {
		return
	}
	var avatarScore struct { // This are the complete list of values that are saved
		TScoreTotal, TScoreBalance float64
		TScoreTime                 uint32
	}
	avatarScore.TScoreTotal = ts.Score
	avatarScore.TScoreBalance = ts.ScoreBalance
	avatarScore.TScoreTime = uint32(ts.TimeStamp.Unix())
	c := db.C("avatars")
	err := c.UpdateId(ts.uid, bson.M{"$set": avatarScore})
	ts.modified = false
	if err != nil {
		log.Println(err)
		return
	}
}
