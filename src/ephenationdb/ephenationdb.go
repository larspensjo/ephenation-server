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
// The purpose of this package is to provide Mongo db access.
// This also provides opportunities to re-use connections, as creating new ones take time.
//

import (
	// "log"
	//	"fmt"
	"labix.org/v2/mgo"
)

var (
	session *mgo.Session
)

// Initialize with connection data.
// The argument is a function that shall provide necessary data for the connection.
func SetConnection(f func(string) string) error {
	login := f("DatabaseLogin")
	pwd := f("DatabasePassword")
	server := f("DatabaseServer")
	database := f("DatabaseName")
	var err error
	session, err = mgo.Dial("mongodb://" + login + ":" + pwd + "@" + server + "/" + database)
	if err == nil {
		session.SetMode(mgo.Strong, true)
	}
	return err
}

// Get a connection.
func New() *mgo.Database {
	return session.DB("")
}
