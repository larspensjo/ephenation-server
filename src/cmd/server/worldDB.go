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

package main

//
// Manage access to the world DB
//
// Chunks use locks to protect against simultaneous access. There are only
// 3 top level functions (that will do the locking), which all calls have to go through:
// DBGetBlock/DBGetBlockCached
// UpdateBlock
// ReadChunkCommand
// There are other functions that manipulate chunks. But these either have to rely
// on the caller doing the lock, or that there can't be any other process
// having access (as is the case for new chunks).
//

import (
	"DynamicBuffer"
	"bytes"
	"chunkdb"
	"client_prot"
	"encoding/gob"
	"fmt"
	sync "github.com/larspensjo/Go-sync-evaluation/evalsync"
	"hash/crc32"
	"io"
	"log"
	"math"
	"os"
	"time"
	"twof"
)

const (
	CHUNK_SIZE = 32                                   // Number of blocks
	CHUNK_VOL  = CHUNK_SIZE * CHUNK_SIZE * CHUNK_SIZE // Just a help constant
)

// These are the block types. Make sure to update blockIsInvisible and blockIsPermeable below.
const (
	BT_Unused        = block(0)
	BT_Stone         = block(1)
	BT_Water         = block(2)
	BT_Air           = block(3)
	BT_Brick         = block(4)
	BT_Soil          = block(5)
	BT_Logs          = block(6)
	BT_Sand          = block(7)
	BT_Tree1         = block(8)  // Bush
	BT_Tree2         = block(9)  // Tree
	BT_Tree3         = block(10) // big tree
	BT_Lamp1         = block(11) // Lamp with little light
	BT_Lamp2         = block(12) // Lamp with much light
	BT_Cobblestone   = block(13)
	BT_Ladder        = block(14)
	BT_Hedge         = block(15)
	BT_Window        = block(16)
	BT_Snow          = block(17)
	BT_BrownWater    = block(18)
	BT_Black         = block(19)
	BT_Concrete      = block(20) // Grey concrete
	BT_WhiteConcrete = block(21) // White concrete
	BT_Gravel        = block(22)
	BT_TiledStone    = block(23)
	BT_SmallFog      = block(24)
	BT_BigFog        = block(25)
	BT_Treasure      = block(26)
	BT_Quest         = block(27)
	BT_Tuft          = block(28)
	BT_Flowers       = block(29)

	BT_Stone2   = block(127)
	BT_Topsoil  = block(128) // This block is never stored in a chunk.
	BT_Teleport = block(129) // This block is never stored in a chunk.

	BT_Text      = block(251) // Generate a text message to anyone near
	BT_DeTrigger = block(252) // The opposite of a trigger. Will reset activator blocks.
	BT_Spawn     = block(253) // Spawn a monster. Activated from a trig block.
	BT_Link      = block(254) // Link a trig action.
	BT_Trigger   = block(255) // Trig an action when a player pass through
)

// Flag for chunks, to be used as bits in a 32-bit unsigned integer
const (
	CHF_MODIFIED = 1 << iota // True if the chunk is modified compared to the randomly generated original
)

// This const group defines partition types (sections saved for every chunk on the file). Be careful of changing
// this numbering if there are saved chunk files.
type TPartition uint16

const (
	PART_COMP_CHUNK      = TPartition(iota) // A compressed chunk
	PART_TEXT_ACTIVATORS = TPartition(iota) // List of text messages associated with text activators in this chunk
)

// This structure is used to associate a trigger with an activation block. It is a many-to-many association.
// For example, if there are 3 triggers and 4 activators all connected, 3x4 BlockTrigger:s are needed.
// This information is recomputed everytime the chunk is loaded from a file or a block is changed in the chunk.
type BlockTrigger struct {
	x, y, z    uint8             // Coordinate of the trigger
	x2, y2, z2 uint8             // Coordinates of an activator
	msg        *textMsgActivator // Pointer to the corresponding text message. Used as a cache to speed up reference.
}

// There is one instance of this struct for each BT_Text block in the chunk. This information is saved with the chunk
// on the file, and only modified when a text block is added or removed.
type textMsgActivator struct {
	X, Y, Z uint8     // Position of the BT_Text inside the chunk
	Message []string  // The multi line message
	inhibit time.Time // The activator is inhibited until this time. Need not be saved with the chunk (thus using lowercase letter).
}

// A jelly block is a block inside a chunk that temporarily turns into air. There is a timer that specifies when the block reverts to
// the original content again.
type jellyBlock struct {
	timeOut  time.Time // When this jelly block should be restored
	original block     // What the original block type was
	x, y, z  uint8     // The block adress inside the chunk
}

// Description of a chunk. A chunk mainly consists of 32x32x32 blocks.
type chunk struct {
	next         *chunk             // Linked list with all chunks at the same hash value
	coord        chunkdb.CC         // The chunk coordinate for this chunk
	rc           *raw_chunk         // nil if no unpacked data. This may be the case now and then, to save RAM.
	ch_comp      []byte             // This pointer always points to something. If no read lock, the pointer may change.
	ch_comp2     []byte             // Same as ch_comp, but all invisible blocks replaced by air. This is only used for sending to clients. Can be nil to save RAM.
	checkSum     uint32             // A checksum for this chunk. This is used by clients to identify when there is a new version of the chunk and the old one has to be discarded.
	flag         uint32             // A bit mapped flag field for this chunk
	blTriggers   []*BlockTrigger    // List of triggers and detriggers, and what they are connected to. This list is recomputed when chunk is restored from file.
	sync.RWMutex                    // Provide read and write mutex.
	owner        uint32             // The owner of this chunk, see OWNER_* below for definitions.
	touched      bool               // Flag used to determine if a chunk can be discarded
	triggerMsgs  []textMsgActivator // List of all activators and their text messages. This list is saved and restored from file.
	jellyBlocks  []jellyBlock       // The current list of jelly blocks. nil when empty. It is sorted in time order, with the first being the oldest.
}

const (
	OWNER_NONE     = 0                     // No owner yet for this chunk
	OWNER_RESERVED = math.MaxUint32 - iota // This chunk will never have an owner
	OWNER_TEST     = math.MaxUint32 - iota // This is a test player, which will normally not be an owner of chunks.
	// When there is more than one test player, they will get subsequent lower numbers
)

// The encoding for a block is stored in this type.
type block uint8

// A chunk is a 3D cube with size 'CHUNK_SIZE'
type raw_chunk [CHUNK_SIZE][CHUNK_SIZE][CHUNK_SIZE]block

// Address of a player. The coordinate is where the player feet are.
type user_coord struct {
	X, Y, Z float64 // Offset from the chunk origin.
}

// Take a user coordinate and get the chunk coordinate
func (uc *user_coord) GetChunkCoord() chunkdb.CC {
	return chunkdb.CC{int32(math.Floor(uc.X / CHUNK_SIZE)),
		int32(math.Floor(uc.Y / CHUNK_SIZE)),
		int32(math.Floor(uc.Z / CHUNK_SIZE))}
}

// Send a command to all near players. This is non blocking. It is not a very fast operation, and should
// not be used for time critcal operations.
// 'cmd': The function pointer that will be invoked for every player
// 'exclude': Do not send to this player
func (coord user_coord) CallNearPlayers_RLq(cmd ClientCommand, exclude *user) {
	near := playerQuadtree.FindNearObjects_RLq(&twof.TwoF{coord.X, coord.Y}, client_prot.NEAR_OBJECTS)

	// Iterate over all near players. We will not find self, as 'up' is not in the quadtree yet.
	for _, o := range near {
		other, ok := o.(*user)
		if !ok {
			log.Println("Not only players in player quad tree")
			continue // Only need to tell players, not monsters etc.
		}
		if exclude == other {
			// This player shall be excluded from the list
			continue
		}
		if other.pl.coord.Z > coord.Z+client_prot.NEAR_OBJECTS || other.pl.coord.Z < coord.Z-client_prot.NEAR_OBJECTS {
			// The quad tree doesn't check for nearness in z dimension
			continue
		}
		other.SendCommand(cmd)
	}
}

func DBChunkFileName(c chunkdb.CC) string {
	return fmt.Sprintf(CnfgChunkFolder+"/%d,%d,%d", c.X, c.Y, c.Z)
}

// Replace all invisible blocks by air, and return the compressed structure
// To be sent to the client.
func (ch *chunk) GetFilteredBlocks() []byte {
	if ch.ch_comp2 != nil {
		return ch.ch_comp2
	}
	ch.ch_comp2 = make([]byte, len(ch.ch_comp))
	for i := 0; i < len(ch.ch_comp); i += 2 {
		// First the type, and then the count. Don't bother with merging invisible blocks into air block counts now.
		if blockIsInvisible[ch.ch_comp[i]] {
			ch.ch_comp2[i] = uint8(BT_Air)
		} else {
			ch.ch_comp2[i] = ch.ch_comp[i]
		}
		ch.ch_comp2[i+1] = ch.ch_comp[i+1]
	}
	return ch.ch_comp2
}

// The chunk is already locked.
func (ch *chunk) Write() {
	// Create file name for this chunk
	fn := DBChunkFileName(ch.coord)
	ch.compress()
	file, err := os.Create(fn)
	if err != nil {
		log.Printf("chunk.Write open %s failed: %v\n", fn, err)
		return
	}
	ch.WriteFS(file)
	file.Close()
}

func (ch *chunk) WriteFS(file io.Writer) bool {
	// Save header data
	var b [24]byte
	EncodeUint32(ch.flag, b[0:4])
	EncodeUint32(ch.checkSum, b[4:8])
	EncodeUint32(ch.owner, b[8:12])
	EncodeUint32(0, b[12:16]) // Reserved for future usage
	EncodeUint32(0, b[16:20]) // Reserved for future usage
	EncodeUint32(0, b[20:24]) // Reserved for future usage
	n, err := file.Write(b[:])
	if err != nil {
		log.Printf("chunk.Write: Saving chunk %v failure: %s (%d of %d bytes)\n", ch.coord, err, n, len(b))
		return false
	}

	// Save the compressed blocks
	err = ch.WritePartition(file, ch.ch_comp, PART_COMP_CHUNK)
	if err != nil {
		log.Printf("chunk.Write: Saving chunk PART_COMP_CHUNK %v failure: %s (%d of %d bytes)\n", ch.coord, err, n, len(b))
		return false
	}

	if len(ch.triggerMsgs) > 0 {
		// Save the activator messages, if there is any, encode using the gob package
		// As the size of the encoded data isn't known, if must be computed before the header is written.
		// A temporary buffer is used for that.
		var buffer bytes.Buffer
		encoder := gob.NewEncoder(&buffer)
		err = encoder.Encode(&ch.triggerMsgs)
		if err != nil {
			log.Printf("WriteFS: encode failed %v (for chunk %v)\n", err, ch.coord)
			return false
		}
		err = ch.WritePartition(file, buffer.Bytes(), PART_TEXT_ACTIVATORS)
		if err != nil {
			log.Printf("WriteFS: PART_TEXT_ACTIVATORS write failed %v (for chunk %v)\n", err, ch.coord)
			return false
		}
	}
	return true
}

func (ch *chunk) WritePartition(file io.Writer, data []byte, pType TPartition) error {
	// fmt.Printf("Write chunk %v partition %v size %v\n", ch.coord, pType, len(data))
	var b [4]byte
	EncodeUint16(uint16(pType), b[0:2])
	EncodeUint16(uint16(len(data)), b[2:4])
	_, err := file.Write(b[:])
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	return nil
}

func dBCreateAndSaveChunk(c chunkdb.CC) *chunk {
	ch := dBCreateChunk(c)
	if c.Y <= 4 && c.Y >= -4 && c.Z <= 2 && c.Z >= -1 {
		ch.owner = OWNER_RESERVED
	}
	ch.Write()
	return ch
}

// No lock needed here, as no other process can access this chunk.
func dBFindChunkFromFS(c chunkdb.CC) *chunk {
	// log.Printf("dBFindChunkFromFS %v\n", c)
	// Create file name for this chunk
	fn := DBChunkFileName(c)
	file, err := os.Open(fn)
	if err != nil {
		// This chunk did not exist yet
		// log.Printf("dBFindChunkFromFS: Chunk %s new, creating it\n", fn)
		return dBCreateAndSaveChunk(c)
	}
	// Chunk found. Load it.
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		log.Printf("dBFindChunkFromFS: Loading chunk %s failure: %s. Creating a new instead.\n", fn, err)
		return dBCreateAndSaveChunk(c)
	}
	// log.Printf("dBFindChunkFromFS: Found %s, size %d\n", fn, fi.Size)
	return dBReadChunk(c, file, fi.Size())
}

var DBStats struct {
	WorstRead time.Duration
	NumRead   int
	TotRead   time.Duration
}

func dBReadChunk(c chunkdb.CC, file io.Reader, size int64) *chunk {
	start := time.Now()
	b := make([]byte, size)
	n, err := file.Read(b)
	if err != nil {
		log.Printf("DBReadChunk: Loading chunk %v failure: %s. Creating a new instead.\n", c, err)
		return dBCreateAndSaveChunk(c)
	}
	if n != int(size) {
		log.Printf("DBReadChunk: Loading chunk %v only got %d(%d) bytes. Creating a new.\n", c, n, size)
		return dBCreateAndSaveChunk(c)
	}
	var ok bool
	ch := new(chunk)
	ch.flag, b, ok = ParseUint32(b)
	if !ok {
		log.Printf("DBReadChunk: ParseUint32 flag failed\n")
		return dBCreateAndSaveChunk(c)
	}
	ch.checkSum, b, ok = ParseUint32(b)
	if !ok {
		log.Printf("DBReadChunk: ParseUint32 checksum failed\n")
		return dBCreateAndSaveChunk(c)
	}
	ch.owner, b, ok = ParseUint32(b)
	if !ok {
		log.Printf("DBReadChunk: ParseUint32 owner failed\n")
		return dBCreateAndSaveChunk(c)
	}
	var pType TPartition
	var pLength uint16
	// If we are not converting old chunk files, then assume it is the "new" format.
	_, b, ok = ParseUint32(b)
	if !ok {
		log.Printf("DBReadChunk: ParseUint32 reserved1 failed\n")
		return dBCreateAndSaveChunk(c)
	}
	_, b, ok = ParseUint32(b)
	if !ok {
		log.Printf("DBReadChunk: ParseUint32 reserved2 failed\n")
		return dBCreateAndSaveChunk(c)
	}
	_, b, ok = ParseUint32(b)
	if !ok {
		log.Printf("DBReadChunk: ParseUint32 reserved3 failed\n")
		return dBCreateAndSaveChunk(c)
	}

	// Iterate through each partition
	for len(b) > 0 {
		var tmp uint16
		tmp, b, ok = ParseUint16(b)
		if !ok {
			log.Printf("DBReadChunk: ParseUint16 partition type failed\n")
			return dBCreateAndSaveChunk(c)
		}
		pType = TPartition(tmp)
		pLength, b, ok = ParseUint16(b)
		if !ok {
			log.Printf("DBReadChunk: ParseUint16 partition length failed\n")
			return dBCreateAndSaveChunk(c)
		}
		if pLength > uint16(len(b)) {
			log.Printf("DBReadChunk: bad partition type %d or partition length %d (%d)\n", pType, pLength, len(b))
			return dBCreateAndSaveChunk(c)
		}
		switch pType {
		case PART_COMP_CHUNK:
			// TODO: The compressed chunk is the slice offset into the whole file. That means there are wasted bytes that will never
			// be used, and not released until the chunk is released. However, doing a copy of the data is maybe expensive.
			ch.ch_comp = b[0:pLength]
			ch.rc = decompressChunk(ch.ch_comp)
			ch.coord = c // Must define the chunk coordinate before following trigger links.
		case PART_TEXT_ACTIVATORS:
			buffer := bytes.NewBuffer(b[0:pLength])
			decoder := gob.NewDecoder(buffer)
			err := decoder.Decode(&ch.triggerMsgs)
			if err != nil {
				log.Printf("DBReadChunk: decode failed %v (from %v)\n", err, b[0:pLength])
				return dBCreateAndSaveChunk(c)
			}
			// fmt.Printf("DBReadChunk ch(%v) activator messages: %v\n", ch.coord, ch.triggerMsgs)
		default:
			log.Printf("DBReadChunk: bad partition type %d or partition length %d (%d)\n", pType, pLength, len(b))
			return dBCreateAndSaveChunk(c)
		}
		b = b[pLength:] // the next partition
	}
	ch.ComputeLinks() // No lock needed yet as the chunk is not available anywhere else
	ch.touched = true // Prevent this chunk from being discarded too soon
	delta := time.Now().Sub(start)
	DBStats.NumRead++
	DBStats.TotRead += delta
	if delta > DBStats.WorstRead {
		DBStats.WorstRead = delta
	}
	return ch
}

// Compress a chunk to save space. This may overwrite the previous compressed data.
func (ch *chunk) compress() {
	// TODO: It is important that the compression algorithm does not waste too much memory,
	// But it must still be quick.
	buff := DynamicBuffer.MakeCompressedBuffer(CHUNK_VOL / 100) // A rough guess for a size
	// Fill this byte array with data
	for x := uint(0); x < CHUNK_SIZE; x++ {
		for y := uint(0); y < CHUNK_SIZE; y++ {
			for z := uint(0); z < CHUNK_SIZE; z++ {
				buff.Add(byte(ch.rc[x][y][z]))
			}
		}
	}
	ch.ch_comp = buff.Bytes()
}

func decompressChunk(ch []byte) *raw_chunk {
	trans := DynamicBuffer.MakeUncompressBuffer(ch)
	rc := new(raw_chunk)
	for x := uint(0); x < CHUNK_SIZE; x++ {
		for y := uint(0); y < CHUNK_SIZE; y++ {
			for z := uint(0); z < CHUNK_SIZE; z++ {
				bl, ok := trans.GetOne()
				b := block(bl)
				if !ok {
					// This should never happen, unless file content is corrupt.
					b = BT_Air
				}
				rc[x][y][z] = b
			}
		}
	}
	return rc
}

// Find the block type at a given user coordinate. This is a speed critical function.
func DBGetBlock_WLwWLc(uc user_coord) block {
	cc := uc.GetChunkCoord()
	cp := ChunkFind_WLwWLc(cc)
	x_off := int32(math.Floor(uc.X)) - cc.X*CHUNK_SIZE
	y_off := int32(math.Floor(uc.Y)) - cc.Y*CHUNK_SIZE
	z_off := int32(math.Floor(uc.Z)) - cc.Z*CHUNK_SIZE
	rc := cp.rc
	return rc[x_off][y_off][z_off]
}

// This function does the same as DBGetBlock(), with the difference that it remembers
// the last chunk that was found. The reason for this is to mimimize time to find the
// chunk again. There are some use cases that iterate over near positions repeatedly.
// This variable can change anytime asynchronously. Always make a copy!
// The pointer is expected to change atomically.
var dbGetBlockLastChunk = &chunk{coord: chunkdb.CC{math.MaxInt32, math.MaxInt32, math.MaxInt32}} // Initialize to invalid chunk address
func ChunkFindCached_WLwWLc(cc chunkdb.CC) *chunk {
	cp := dbGetBlockLastChunk // Make a copy of the pointer to the previous chunk used
	if cp.coord.X != cc.X || cp.coord.Y != cc.Y || cp.coord.Z != cc.Z {
		cp = ChunkFind_WLwWLc(cc)
		dbGetBlockLastChunk = cp
	} else {
		if cp.jellyBlocks != nil {
			// There may be jelly blocks that should be restored
			cp.Lock()
			// It may have changed between the test and the lock, but minimal chance
			cp.RestoreJellyBlocks(false)
			cp.Unlock()
		}
	}
	return cp
}

// Get block type at specified coordinate. Use cached chunk pointer to speed up.
// It is a snap shot value, that can change afterwards.
func DBGetBlockCached_WLwWLc(uc user_coord) block {
	cc := uc.GetChunkCoord()
	cp := ChunkFindCached_WLwWLc(cc)
	x_off := int32(math.Floor(uc.X)) - cc.X*CHUNK_SIZE
	y_off := int32(math.Floor(uc.Y)) - cc.Y*CHUNK_SIZE
	z_off := int32(math.Floor(uc.Z)) - cc.Z*CHUNK_SIZE
	rc := cp.rc
	dbGetBlockLastChunk = cp // Save this chunk pointer for next
	return rc[x_off][y_off][z_off]
}

// The checksum is computed on the compressed data. This means that the data must be compressed first.
func (this *chunk) updateChecksum() {
	this.checkSum = crc32.ChecksumIEEE(this.ch_comp)
}

// The update of the chunk should possibly be done by a worldDB process.
func (cp *chunk) UpdateBlock_WLcWLw(x_off, y_off, z_off uint8, blType block) bool {
	cp.Lock()
	defer cp.Unlock()
	if cp.jellyBlocks != nil {
		cp.RestoreJellyBlocks(true)
	}
	rc := cp.rc
	if rc[x_off][y_off][z_off] != BT_Air && blType != BT_Air {
		// Non fatal problem, a client maybe tried twice.
		log.Printf("UpdateBlock (%d,%d,%d) chunk %v had type %d already\n", x_off, y_off, z_off, cp.coord, blType)
		return false
	}

	rc[x_off][y_off][z_off] = blType
	cp.compress()       // Make a new compressed chunk
	cp.updateChecksum() // Update the time stamp
	cp.flag |= CHF_MODIFIED
	// Save it permanently. TODO: Use delayed write to improve performance. The reason for this is
	// that the chunk usually updates again. However, a delayed compress can't be used as that would delay the
	// checksum, which must be updated before the function is ended.
	if !*inhibitCreateChunks {
		cp.Write()
	}
	cp.ComputeLinks()
	return true
}

// Turn one block to jelly (transparent and permeable), and set the timer for he it shall be
// reverted.
// The chunk must be write locked.
func (cp *chunk) TurnToJelly(x, y, z uint8, timeout time.Time) {
	orig := cp.rc[x][y][z]
	if orig == BT_Air {
		log.Println("Tried to make jelly of air at", cp.coord, x, y, z)
		return
	}
	jb := jellyBlock{x: x, y: y, z: z, original: orig, timeOut: timeout}
	cp.jellyBlocks = append(cp.jellyBlocks, jb)
	cp.rc[x][y][z] = BT_Air
}

// Look at the list of all jelly blocks and revert those that have timed out.
// The block must be write locked.
func (cp *chunk) RestoreJellyBlocks(unconditionally bool) {
	remain := 0 // Index of the first jelly block that shall remain in the list
	now := time.Now()
	jb := cp.jellyBlocks
	for i, j := range jb {
		if j.timeOut.After(now) && !unconditionally {
			break
		}
		cp.rc[j.x][j.y][j.z] = j.original
		remain = i + 1
	}
	jb = jb[remain:] // Remove all jelly blocks that have timed out.
	if len(jb) == 0 {
		// Release the list as it is now empty.
		jb = nil
	}
	if *verboseFlag > 1 && remain > 0 {
		log.Println("RestoreJellyBlocks from", cp.jellyBlocks, "to", jb, "now:", now)
	}
	cp.jellyBlocks = jb
}

// Investigate if the player coord "c" is a valid place to spawn at. The 'height' is
// measured in blocks.
func ValidSpawnPoint_WLwWLc(c user_coord, height float64) bool {
	if c.Z < 0 {
		// Too low
		return false
	}
	// There must be stable ground below on the block below the feet
	if blockIsPermeable[DBGetBlockCached_WLwWLc(user_coord{c.X, c.Y, c.Z - 1})] {
		return false
	}
	// Check that there is empty space available above
	for i := float64(0); i < height; i++ {
		if !blockIsPermeable[DBGetBlockCached_WLwWLc(user_coord{c.X, c.Y, c.Z + i})] {
			return false
		}
	}
	return true
}

var (
	// The purpose of these arrauys is to quickly be able to determine properties of blocks
	blockIsInvisible [256]bool // These will not be shown normally
	blockIsPermeable [256]bool // You can walk through these blocks
)

func init() {
	blockIsInvisible[BT_Air] = true
	blockIsInvisible[BT_DeTrigger] = true
	blockIsInvisible[BT_Spawn] = true
	blockIsInvisible[BT_Link] = true
	blockIsInvisible[BT_Trigger] = true
	blockIsInvisible[BT_Text] = true
	blockIsInvisible[BT_SmallFog] = true
	blockIsInvisible[BT_BigFog] = true

	blockIsPermeable[BT_Air] = true
	blockIsPermeable[BT_DeTrigger] = true
	blockIsPermeable[BT_Spawn] = true
	blockIsPermeable[BT_Link] = true
	blockIsPermeable[BT_Trigger] = true
	blockIsPermeable[BT_Text] = true
	blockIsPermeable[BT_Water] = true
	blockIsPermeable[BT_BrownWater] = true
	blockIsPermeable[BT_Tree1] = true
	blockIsPermeable[BT_Tree2] = true
	blockIsPermeable[BT_Tree3] = true
	blockIsPermeable[BT_Flowers] = true
	blockIsPermeable[BT_Tuft] = true
	blockIsPermeable[BT_SmallFog] = true
	blockIsPermeable[BT_BigFog] = true
	blockIsPermeable[BT_Treasure] = true
	blockIsPermeable[BT_Quest] = true
}

// Update the Z position of an object, taking into account ground level and z speed. The argument is updated,
// and the new z speed is returned.
// TODO: This algorihm can be optimized
func UpdateZPos_WLwWLc(deltaTime time.Duration, ZSpeed float64, coord *user_coord) (newZSpeed float64) {
	newZSpeed = ZSpeed - float64(deltaTime)*GRAVITY/1e9 // Accelerate downwards
	if newZSpeed > 0 {
		// TODO: Do not allow jumping through the roof of a tunnel
		coord.Z += newZSpeed
	} else if newZSpeed < 0 {
		// Falling downwards. If moving more than one block, be careful not to fall through a block.
		zspeed := -newZSpeed                 // The downward speed, which is now a value bigger than 0
		start := zspeed - math.Floor(zspeed) // Get the decimals
		var ok float64 = 0                   // The eventual allowance to fall downward, limited from zspeed
		testCoord := *coord
		// Test for presence of ground every block, until player has been moved according to
		// the current downward speed.
		for d := start; d <= zspeed; d += 1 {
			testCoord.Z = coord.Z - d
			// fmt.Printf("Check d %d, z %d\n", d, testCoord.z)
			if !blockIsPermeable[DBGetBlockCached_WLwWLc(testCoord)] {
				// Stop the fall, move player to above ground.
				ok = coord.Z - math.Floor(testCoord.Z) - 1
				newZSpeed = 0
				// fmt.Printf("Ground, ok = %d, coord.Z = %d, round = %d\n", ok, coord.Z,
				break
			}
			ok = d
		}
		coord.Z -= ok
	}
	return
}

// Find the 6 adjacent chunks to a chunk coordinate.
// Never call this from the chunk loading or creation functions, as infinite recursion can happen.
func dBGetAdjacentChunks(cc *chunkdb.CC) []*chunk {
	var ret [6]*chunk
	ret[0] = ChunkFind_WLwWLc(chunkdb.CC{cc.X + 1, cc.Y, cc.Z})
	ret[1] = ChunkFind_WLwWLc(chunkdb.CC{cc.X - 1, cc.Y, cc.Z})
	ret[2] = ChunkFind_WLwWLc(chunkdb.CC{cc.X, cc.Y + 1, cc.Z})
	ret[3] = ChunkFind_WLwWLc(chunkdb.CC{cc.X, cc.Y - 1, cc.Z})
	ret[4] = ChunkFind_WLwWLc(chunkdb.CC{cc.X, cc.Y, cc.Z + 1})
	ret[5] = ChunkFind_WLwWLc(chunkdb.CC{cc.X, cc.Y, cc.Z - 1})
	return ret[:]
}

// Set a teleport in the specified chunk.
func (cp *chunk) SetTeleport(cc chunkdb.CC, up *user, x, y, z uint8) {
	if cp == nil || (cp.owner != up.uid && up.pl.adminLevel == 0) {
		up.Printf_Bl("#FAIL")
		return
	}

	// Count the number of teleports in other chunks that the player already has
	numTeleports := 0
	for _, terr := range up.pl.territory {
		_, _, _, found := superChunkManager.GetTeleport(&terr)
		if found && terr != cc {
			numTeleports++
			// up.Printf_Bl("%v", terr)
		}
	}
	if numTeleports > 0 && up.pl.adminLevel == 0 {
		up.Printf_Bl("#FAIL You can only have one magical portal")
		return
	}
	prevX, prevY, prevZ, foundOld := superChunkManager.GetTeleport(&cc)
	superChunkManager.SetTeleport(&cc, x, y, z)
	f := func(up *user) {
		up.SuperChunkAnswer_Bl(&cc)
		if foundOld {
			up.SendMessageBlockUpdate(cc, prevX, prevY, prevZ, BT_Air)
		}
		up.SendMessageBlockUpdate(cc, x, y, z, BT_Teleport)
	}
	up.pl.coord.CallNearPlayers_RLq(f, nil)
}
