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

package keys

// This package manages keys. The keys are maintained in a key ring, which has a limited size.
// If new keys are added, old are dropped.

const (
	keyRingMaxSize = 10
)

type keyDefinition struct {
	Uid   uint32 // This is the territory owner that created the key.
	Kid   uint   // The unique id of the key, for the given territory owner.
	Descr string // A one line description of the key.
	View  uint   // How it looks. This is used by the client to choose a display.
}

// The list of keys
type KeyRing []*keyDefinition

// Add a key to the key ring
func (kr KeyRing) Add(key *keyDefinition) KeyRing {
	if kr.Test(key.Uid, key.Kid) {
		// Already have the key
		return kr
	}
	if len(kr) >= keyRingMaxSize {
		// The keyring is now too big. Throw away the oldest one.
		kr = kr[1:]
	}
	kr = append(kr, key)
	return kr
}

// Test if a key is in the key ring
func (kr KeyRing) Test(Uid uint32, Kid uint) bool {
	for _, key := range kr {
		if key.Kid == Kid && key.Uid == Uid {
			return true
		}
	}
	return false
}

// Make a key
func Make(uid uint32, kid uint, descr string, view uint) *keyDefinition {
	return &keyDefinition{uid, kid, descr, view}
}
