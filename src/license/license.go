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

package license

import (
	"fmt"
	"io"
	//	"os"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"ephenationdb"
	"flag"
	"log"
	"time"
)

var verboseFlag = flag.Int("license.v", 0, "Debug license management, Higher number gives more")

const (
	keyLength        = 20 // Use 20 character for a license key
	UPPER_TIME_LIMIT = 0  // A load/save operation longer than this (in ns) will generate a log message
)

// All this data is saved with the license. All names beginning with upper case will be
// saved to file.
type License struct {
	mail     string   // This isn't saved, but used for the file name.
	Password string   // The password of the license
	License  string   // The license for this person
	LastSeen string   // Date when this player was last logged on
	Names    []string // Array of player names for this license. Only one name is used for now.
}

// Load a license. Return nil if error.
func Load_Bl(mail string) *License {
	var license License

	start := time.Now()
	db := ephenationdb.New()
	if db == nil {
		return nil
	}

	// Build a query for the user email sent as an argument
	query := "SELECT * FROM users WHERE email='" + mail + "'"
	if *verboseFlag > 0 {
		log.Println(query)
	}
	err := db.Query(query)
	if err != nil {
		log.Println(err)
		return nil
	}

	defer ephenationdb.Release(db) // Defer database release when we are sure that the db is open

	// Store the result
	result, err := db.UseResult()
	if err != nil {
		log.Println(err)
		return nil
	}

	// Fetch row
	row := result.FetchRow()
	if row == nil {
		if *verboseFlag > 0 {
			log.Printf("No result for %v\n", mail)
		}
		db.FreeResult()
		return nil
	}

	// Assign mail as license.mail
	license.mail = mail
	idnumfields := 0

	const (
		ID_MAIL  = 1 << iota
		ID_LIC   = 1 << iota
		ID_PASSW = 1 << iota
		ID_LAST  = 1 << iota
	)

	// TODO: Read more information: license type, valid until (if license terminates during connection)
	for x := 0; x < int(result.FieldCount()); x++ {
		FieldName := result.FetchField().Name
		switch FieldName {
		case "email":
			license.mail = fmt.Sprint(row[x])
			idnumfields |= ID_MAIL
		case "licensekey":
			license.License = fmt.Sprint(row[x])
			idnumfields |= ID_LIC
		case "password":
			license.Password = fmt.Sprint(row[x])
			idnumfields |= ID_PASSW
		case "lastseen":
			license.LastSeen = fmt.Sprintf("%s", row[x])
			idnumfields |= ID_LAST
		}
	}

	db.FreeResult() // Required to get back in sync

	// If all fields have been identified, continue, otherwise return failure
	if idnumfields != ID_MAIL|ID_LIC|ID_PASSW|ID_LAST {
		if *verboseFlag > 0 {
			log.Printf("Not proper content: %v\n", idnumfields)
		}
		return nil // return fail
	}

	// Read avatar names
	query = "SELECT name FROM avatars WHERE owner='" + license.mail + "'"
	if *verboseFlag > 0 {
		log.Println(query)
	}
	err = db.Query(query)
	if err != nil {
		log.Println(err)
		return nil
	}

	// Store the result
	result, err = db.StoreResult()
	if err != nil {
		log.Println(err)
		return nil
	}

	rowcount := result.RowCount()
	temps := make([]string, rowcount)

	for x := 0; x < int(rowcount); x++ {
		// Fetch row
		row = result.FetchRow()
		if row == nil {
			// TODO: Error handling
		}
		temps[x] = fmt.Sprint(row[0])
	}

	db.FreeResult()

	license.Names = temps

	if *verboseFlag > 0 {
		log.Printf("license.Load done: %v\n", license)
		elapsed := time.Now().Sub(start)
		if elapsed > UPPER_TIME_LIMIT {
			log.Printf("license.Load %d ms\n", elapsed/1e6)
		}
	}

	return &license
}

// #258 update user last seen
func (lp *License) SaveLogonTime_Bl() {
	start := time.Now()
	db := ephenationdb.New()
	if db == nil {
		log.Println("license.SaveLogonTime: Failed to save logon time")
		return
	}

	defer ephenationdb.Release(db) // Defer database close when we are sure that the db is open

	// Update last seen online --> Moved from Save() due to issue #87: LastSeen not updated in database
	now := time.Now()
	nowstring := fmt.Sprintf("%4v-%02v-%02v", now.Year(), int(now.Month()), now.Day())

	query := "UPDATE users SET lastseen='" + nowstring + "' WHERE email='" + lp.mail + "'"
	//fmt.Printf("%v\n",query)
	if *verboseFlag > 0 {
		log.Println(query)
	}
	err := db.Query(query)

	if err != nil {
		log.Println(err)
		return
	}

	elapsed := time.Now().Sub(start)
	// if elapsed > UPPER_TIME_LIMIT && *verboseFlag > 0 {
	log.Printf("license.SaveLogonTime %d ms\n", elapsed/1e6)
	// }
	return
}

func (lp *License) Save_Bl() bool {
	start := time.Now()
	db := ephenationdb.New()
	if db == nil {
		return false
	}

	defer ephenationdb.Release(db) // Defer database close when we are sure that the db is open

	// Check if the user exists
	query := "SELECT * FROM users WHERE email='" + lp.mail + "'"
	err := db.Query(query)
	if err != nil {
		log.Println(err)
		return false
	}

	// Store the result
	result, err := db.StoreResult()
	if err != nil {
		log.Println(err)
		return false
	}

	if result.RowCount() == 0 {
		db.FreeResult()

		query = "INSERT INTO users SET email='" + lp.mail + "', password='" + lp.Password + "', isvalidated=1, "
		query = query + "licensekey='" + lp.License + "'"

		err = db.Query(query)
	} else {
		db.FreeResult()

		// Update password
		// TODO: check if password has changed? Add parameter in struct to know if changed or not?
		query = "UPDATE users SET password='" + lp.Password + "' WHERE email='" + lp.mail + "'"
		err = db.Query(query)
		if err != nil {
			log.Println(err)
			return false
		}

	}
	elapsed := time.Now().Sub(start)
	// if elapsed > UPPER_TIME_LIMIT && *verboseFlag > 0 {
	log.Printf("license.Save %d ms\n", elapsed/1e6)
	// }
	return true
}

// Compare the given password with the stored one. The stored password
// is scrambled using md5, and so is never available as readable text.
func (lp *License) VerifyPassword(passw string, salt string) bool {
	hash := md5.New()
	_, err := io.WriteString(hash, salt+passw)
	if err != nil {
		panic("license.VerifyPassword write to hash")
	}
	// fmt.Printf("license.VerifyPassword: '%s' - '%s'\n", hex.EncodeToString(hash.Sum()), lp.Password)
	if hex.EncodeToString(hash.Sum(nil)) == lp.Password {
		return true
	}
	return false
}

// Update the password with a new one. The stored password
// is scrambled using md5, and so is never available as readable text.
func (lp *License) NewPassword(passw string) {
	hash := md5.New()
	_, err := io.WriteString(hash, passw)
	if err != nil {
		panic("license.NewPassword write to hash")
	}
	// fmt.Println("NewPassword hash", hash.Sum())
	lp.Password = hex.EncodeToString(hash.Sum(nil))
}

func GenerateKey() string {
	// This list of characters is not very important. Some were excluded as it can be hard to read the difference between '1' and 'I', etc.
	const keyCharacters = "ABCDEFGHIJKLMNPQRSTUVXYZ23456789abcdefghijklmnpqrstuvxyz"
	var str [keyLength]byte
	var b [4]byte
	for i := 0; i < keyLength; i++ {
		var rnd uint
		rand.Read(b[:])
		for j := 0; j < 4; j++ {
			rnd = (rnd << 8) + uint(b[j])
		}
		str[i] = keyCharacters[rnd%uint(len(keyCharacters))]
	}
	return string(str[:])
}

func Make(mail string) *License {
	return &License{mail: mail}
}
