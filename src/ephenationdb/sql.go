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

package ephenationdb

//
// The purpose of this package is to provide SQL db access.
// This also provides opportunities to re-use connections, as creating new ones take time.
//

import (
	sync "evalsync"
	"log"
	"mysql"
	//	"fmt"
	"time"
)

var (
	cachedb            *mysql.Client
	mutex              sync.RWMutex // This lock shall be active when the local cache is accessed
	released           time.Time    // When the current db, if any, was released
	countFailure       int
	dbLicenseDatabase  string
	dbDatabaseName     string
	dbDatabaseLogin    string
	dbDatabasePassword string
)

const (
	maxRetries = 5 // Number of times a failure with report an error. Go silent after that.
)

// Initialize with connection data
func SetConnection(server, name, login, pwd string) {
	dbLicenseDatabase = server
	dbDatabaseName = name
	dbDatabaseLogin = login
	dbDatabasePassword = pwd
}

// Get a connection. Notice that this shall not be called from initialization thread.
func New() *mysql.Client {
	mutex.Lock()
	db := cachedb
	now := time.Now()
	if db != nil {
		// There was a cached db connection, use it.
		cachedb = nil
	}
	if db != nil && now.Sub(released) > 1e11 {
		// The saved db was too old, discard it as we don't trust it
		// TODO: Better would be a way to test if it is still valid, but it must not take time.
		db = nil
		// log.Println("Discard the old db")
	}
	mutex.Unlock()
	if db == nil {
		// There was no local cache to be used
		var err error
		db, err = mysql.DialTCP(dbLicenseDatabase, dbDatabaseLogin, dbDatabasePassword, dbDatabaseName)
		if err != nil {
			countFailure++
			if countFailure == maxRetries {
				log.Println(err, "(last warning)")
			} else if countFailure < maxRetries {
				log.Println(err)
			}
		} else {
			countFailure = 0
		}
	}
	return db
}

// Release a connection. After release, it must not be used again.
func Release(db *mysql.Client) {
	mutex.Lock()
	if cachedb != nil {
		// Not able to save any more client connections, delete the old one
		cachedb.Close()
	}
	// Save the returned client connection, to be reused
	cachedb = db
	released = time.Now()
	mutex.Unlock()
}
