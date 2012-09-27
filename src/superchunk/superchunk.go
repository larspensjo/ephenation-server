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

// This is a package that will maintain teleport locations.
// The can be at most one TP for each chunk.
// Data will be saved into the local file system. To optimize storage, information about
// 10x10x10 chunks are stored in each file.

import (
	. "chunkdb"
	sync "evalsync"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

const (
	SCH_SIZE = 10 // This cannot be changed afterwards, if there are stored super chunks.
)

// Definition of bits in the flag field
const (
	flagTPDefined uint8 = 0x01
)

type superChunkManager struct {
	loaded    map[CC]*superChunk // All superChunks loaded in memory
	lock      sync.RWMutex       // A RWMutex to protect against simultaneous access
	subFolder string
}

// Make sure the required sub folder exists
func New(subFolder string) *superChunkManager {
	stat, err := os.Stat(subFolder)
	if err != nil {
		os.Mkdir(subFolder, 0777)
	} else if !stat.Mode().IsDir() {
		log.Panicln("Bad folder", subFolder)
		return nil
	}
	return &superChunkManager{loaded: make(map[CC]*superChunk), subFolder: subFolder}
}

// Information about one specific chunk
type chunkData struct {
	flag    uint8
	x, y, z uint8
}

// This type is what will be saved to file. The array is a 3D sub set of chunks. The chunk at 0,0,0 is always one with
// the chunk coordinate rounded down to a near multiple of SCH_SIZE.
type superChunk struct {
	checksum uint32 // A simplified value, which is really a counter that change everytime there is a change
	chunk    [SCH_SIZE][SCH_SIZE][SCH_SIZE]chunkData
}

// The file name used for storing a superChunk.
func (scm *superChunkManager) functionName(cc *CC) string {
	return fmt.Sprintf("%s/s%d,%d,%d", scm.subFolder, cc.X, cc.Y, cc.Z)
}

// The chunk coordinate is expected to be truncated to nearest super chunk address
func (scm *superChunkManager) load(cc *CC) *superChunk {
	fn := scm.functionName(cc)
	d, err := ioutil.ReadFile(fn)
	if err != nil {
		// No file found
		return nil
	}
	if len(d) != 4+SCH_SIZE*SCH_SIZE*SCH_SIZE*4 {
		log.Println("Bad file size ", len(d))
		return nil
	}
	var sc superChunk
	sc.checksum = uint32(d[0]) + uint32(d[1])<<8 + uint32(d[2])<<16 + uint32(d[3])<<24
	ind := 4
	for x := 0; x < SCH_SIZE; x++ {
		for y := 0; y < SCH_SIZE; y++ {
			for z := 0; z < SCH_SIZE; z++ {
				sc.chunk[x][y][z].flag = d[ind]
				ind++
				sc.chunk[x][y][z].x = d[ind]
				ind++
				sc.chunk[x][y][z].y = d[ind]
				ind++
				sc.chunk[x][y][z].z = d[ind]
				ind++
			}
		}
	}
	return &sc
}

// Do the actual writing of data
func (sc *superChunk) write(writer io.Writer) {
	var d [4]byte
	d[0] = uint8(sc.checksum)
	d[1] = uint8(sc.checksum >> 8)
	d[2] = uint8(sc.checksum >> 16)
	d[3] = uint8(sc.checksum >> 24)
	writer.Write(d[0:4])
	for x := 0; x < SCH_SIZE; x++ {
		for y := 0; y < SCH_SIZE; y++ {
			for z := 0; z < SCH_SIZE; z++ {
				d[0] = sc.chunk[x][y][z].flag
				d[1] = sc.chunk[x][y][z].x
				d[2] = sc.chunk[x][y][z].y
				d[3] = sc.chunk[x][y][z].z
				writer.Write(d[0:4])
			}
		}
	}
}

// Save a super chunk
func (scm *superChunkManager) save(cc *CC, sc *superChunk) {
	fn := scm.functionName(cc)
	file, err := os.Create(fn)
	if err != nil {
		log.Println("Failed to create", fn)
		return
	}
	sc.write(file)
	file.Close()
}

// Round a number down to nearest multiple of SCH_SIZE
func trunc(a int32) int32 {
	if a < 0 {
		a -= SCH_SIZE - 1
	}
	return (a / SCH_SIZE) * SCH_SIZE
}

// Find the specified superchunk. The chunk coordinate must be a base coordinate.
// It will always succeed, and create a new super chunk if it didn't exist.
func (scm *superChunkManager) findSuperChunk(cc *CC) *superChunk {
	// Look for it in memory
	scm.lock.RLock()
	sc := scm.loaded[*cc]
	scm.lock.RUnlock()

	if sc != nil {
		// log.Println("Found", *cc)
		return sc
	}

	// It wasn't loaded into memory, try from file
	// There is a small chance this is done at the same time from several places. Worst case
	// result will be that the same file will be loaded twice.
	sc = scm.load(cc)

	// Not found, create one
	if sc == nil {
		log.Println("Creating new", cc)
		sc = new(superChunk)
		scm.lock.Lock()
		sc2 := scm.loaded[*cc]
		// Safety test, someone else may have created this superchunk before it was locked
		if sc2 != nil {
			sc = sc2
		} else {
			scm.loaded[*cc] = sc
		}
		scm.lock.Unlock()
	}

	// log.Println(*cc)
	scm.lock.Lock()
	scm.loaded[*cc] = sc
	scm.lock.Unlock()

	return sc
}

// Get the the teleport coordinate for the specified chunk coord.
// 4:th argument is true iff there is one
func (scm *superChunkManager) GetTeleport(cc *CC) (uint8, uint8, uint8, bool) {
	ccBase := CC{trunc(cc.X), trunc(cc.Y), trunc(cc.Z)}
	sc := scm.findSuperChunk(&ccBase)
	ch := &sc.chunk[cc.X-ccBase.X][cc.Y-ccBase.Y][cc.Z-ccBase.Z]
	return ch.x, ch.y, ch.z, ch.flag&flagTPDefined != 0
}

// For Chunk 'cc', set the teleport localtion to x,y,z.
func (scm *superChunkManager) SetTeleport(cc *CC, x, y, z uint8) {
	ccBase := CC{trunc(cc.X), trunc(cc.Y), trunc(cc.Z)}
	sc := scm.findSuperChunk(&ccBase)
	ch := &sc.chunk[cc.X-ccBase.X][cc.Y-ccBase.Y][cc.Z-ccBase.Z]
	ch.flag |= flagTPDefined
	ch.x = x
	ch.y = y
	ch.z = z
	log.Println("SetTeleport", ccBase, ch)
	scm.lock.Lock()
	sc.checksum++
	scm.save(&ccBase, sc)
	scm.lock.Unlock()
}

func (scm *superChunkManager) RemoveTeleport(cc *CC) {
	ccBase := CC{trunc(cc.X), trunc(cc.Y), trunc(cc.Z)}
	sc := scm.findSuperChunk(&ccBase)
	ch := &sc.chunk[cc.X-ccBase.X][cc.Y-ccBase.Y][cc.Z-ccBase.Z]
	ch.flag &= ^flagTPDefined
	ch.x = 0
	ch.y = 0
	ch.z = 0
	log.Println("RemoveTeleport", ccBase, cc)
	scm.lock.Lock()
	sc.checksum++
	scm.save(&ccBase, sc)
	scm.lock.Unlock()
}

// Write the base Chunk Coord address, and then the super chunk
func (scm *superChunkManager) Write(writer io.Writer, cc *CC) {
	ccBase := CC{trunc(cc.X), trunc(cc.Y), trunc(cc.Z)}
	sc := scm.findSuperChunk(&ccBase)
	// This is done without a lock. There is a risk for asynchronous changes, which are accepted.
	// log.Println("Sending", ccBase)
	var msg [3]byte
	msg[0] = byte(ccBase.X & 0xFF)
	msg[1] = byte(ccBase.Y & 0xFF)
	msg[2] = byte(ccBase.Z & 0xFF)
	writer.Write(msg[:])
	sc.write(writer)
}

// Verify that the checksum is correct
func (scm *superChunkManager) VerifyChecksum(cc *CC, checksum uint32) bool {
	ccBase := CC{trunc(cc.X), trunc(cc.Y), trunc(cc.Z)}
	sc := scm.findSuperChunk(&ccBase)
	// log.Println("VerifyChecksum for", cc, sc.checksum, checksum)
	return sc.checksum == checksum
}

func (scm *superChunkManager) Size() int {
	return len(scm.loaded)
}
