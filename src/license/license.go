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
	"flag"
	"io"
)

var verboseFlag = flag.Int("license.v", 0, "Debug license management, Higher number gives more")

const (
	keyLength        = 20 // Use 20 character for a license key
	UPPER_TIME_LIMIT = 0  // A load/save operation longer than this (in ns) will generate a log message
)

// All this data is saved with the license (user DB). All names beginning with upper case will be saved.
type License struct {
	Email    string
	Password string // The password of the license
	License  string // The license key for this person
}

// Compare the given password with the stored one. The stored password
// is scrambled using md5, and so is never available as readable text.
func VerifyPassword(passwclear, passwordcrypted string, salt string) bool {
	// fmt.Printf("license.VerifyPassword: '%s' - '%s'\n", hex.EncodeToString(hash.Sum()), lp.Password)
	if EncryptPassword(passwclear, salt) == passwordcrypted {
		return true
	}
	return false
}

// Update the password with a new one. The stored password
// is scrambled using md5, and so is never available as readable text.
func EncryptPassword(passw, salt string) string {
	hash := md5.New()
	_, err := io.WriteString(hash, passw)
	if err != nil {
		panic("license.NewPassword write to hash")
	}
	return hex.EncodeToString(hash.Sum(nil))
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

// Use a license key and a password
func Make(password, salt string) (string, string) {
	licence := GenerateKey()
	EncryptPassword := EncryptPassword(password, salt)
	return licence, EncryptPassword
}
