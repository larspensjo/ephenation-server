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

package superchunk

import (
	. "chunkdb"
	"os"
	"testing"
	// "time"
)

const SCH_SUBFOLDER = "SDB"

func TestSuperchunk(t *testing.T) {

	stat, err := os.Stat(SCH_SUBFOLDER)
	if err != nil || !stat.Mode().IsDir() {
		// The folder didn't exist, so create it.
		os.Mkdir(SCH_SUBFOLDER, os.ModePerm)
	}

	scm := New(SCH_SUBFOLDER)

	// Loading non existing chunk shall fail
	unused := CC{1, 1, 1}
	s := scm.load(&unused)
	if s != nil {
		t.Error("Unexpected finding super chunk at illegal address")
	}

	cc := CC{11, 12, 13} // Counting on this chunk not existing
	_, _, _, ok := scm.GetTeleport(&cc)
	if ok {
		t.Error("Load", cc, "should have failed")
	}

	scm.SetTeleport(&unused, 1, 2, 3)
	x, y, z, ok := scm.GetTeleport(&unused)
	if !ok {
		t.Error("Failed to read back", unused)
	}
	if x != 1 || y != 2 || z != 3 {
		t.Error("Got bad data back", x, y, z)
	}

	scm.RemoveTeleport(&unused)
	_, _, _, ok = scm.GetTeleport(&unused)
	if ok {
		t.Error("Still set after remove", unused)
	}

	ccBase := CC{trunc(unused.X), trunc(unused.Y), trunc(unused.Z)}
	fn := scm.functionName(&ccBase)
	err = os.Remove(fn)
	if err != nil {
		t.Error("Failed to remove", fn, err)
	}
}
