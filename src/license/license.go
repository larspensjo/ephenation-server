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
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"ephenationdb"
	"flag"
	"fmt"
	"io"
	"labix.org/v2/mgo/bson"
	"log"
	"time"
)

var verboseFlag = flag.Int("license.v", 0, "Debug license management, Higher number gives more")

const (
	keyLength        = 20 // Use 20 character for a license key
	UPPER_TIME_LIMIT = 0  // A load/save operation longer than this (in ns) will generate a log message
)

// All this data is saved with the license (user DB). All names beginning with upper case will be saved.
type License struct {
	Mail     string `bson:"_id"`
	Password string // The password of the license
	License  string // The license for this person
	LastSeen string // Date when this player was last logged on
}

// Load a license. Return nil if error.
func Load_Bl(mail string) *License {
	var license License

	start := time.Now()
	db := ephenationdb.New()
	err := db.C("users").FindId(mail).One(&license)

	if err != nil {
		log.Println("User", mail, err)
		return nil
	}

	if *verboseFlag > 0 {
		log.Printf("license.Load done: %v\n", license)
		elapsed := time.Now().Sub(start)
		if elapsed > UPPER_TIME_LIMIT {
			log.Printf("license.Load %d ms\n", elapsed/1e6)
		}
	}

	return &license
}

// Update user last seen
func (lp *License) SaveLogonTime_Bl() {
	now := time.Now()
	nowstring := fmt.Sprintf("%4v-%02v-%02v", now.Year(), int(now.Month()), now.Day())
	db := ephenationdb.New()
	c := db.C("users")
	err := c.UpdateId(lp.Mail, bson.M{"$set": bson.M{"lastseen": nowstring}})
	if err != nil {
		log.Println("Update", lp.Mail, ":")
		return
	}

	elapsed := time.Now().Sub(now)
	// if elapsed > UPPER_TIME_LIMIT && *verboseFlag > 0 {
	log.Printf("license.SaveLogonTime %d ms\n", elapsed/1e6)
	// }
	return
}

func (lp *License) Save_Bl() bool {
	start := time.Now()
	db := ephenationdb.New()
	c := db.C("users")
	_, err := c.UpsertId(lp.Mail, lp)
	if err != nil {
		log.Println("UsertId failed", lp, ":", err)
		return false
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

// Use a mail and a password, and create a license
func Make(mail, password string) *License {
	lic := &License{Mail: mail, License: GenerateKey()}
	lic.NewPassword(password)
	return lic
}
