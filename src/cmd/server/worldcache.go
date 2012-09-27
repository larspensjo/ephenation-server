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
// All chunks loaded in memory are managed by a hash table to make them quick and
// easy to find.
// TODO: A mechanism is needed to throw away chunks when there are too many
//

import (
	// "fmt"
	"chunkdb"
	sync "evalsync"
)

const (
	WORLD_CACHE_SIZE = 32767 // An even number, like 1000, doesn't work good with the random number
)

var (
	world_cache                [WORLD_CACHE_SIZE]*chunk
	worldCacheLock             sync.RWMutex
	cache_average_search_depth float32 // Keep track of how much searching is needed
	worldCacheNumChunks        int
)

const cache_depth_decay = 0.99

func chunkFindHash(coord chunkdb.CC) uint {
	hash := uint(coord.X)*871 + uint(coord.Y)*988261 + uint(coord.Z)*79261
	return hash % WORLD_CACHE_SIZE
}

// Find the chunk, if it exists. Otherwise, return nil. There is already a lock in place for the cache.
func chunkFindRO(coord chunkdb.CC) *chunk {
	ind := chunkFindHash(coord)
	var pc *chunk
	var n int
	for pc = world_cache[ind]; pc != nil; pc = pc.next {
		n++ // Number of searches
		if pc.coord.X == coord.X && pc.coord.Y == coord.Y && pc.coord.Z == coord.Z {
			pc.touched = true
			break
		}
	}
	cache_average_search_depth = cache_depth_decay*cache_average_search_depth + (1-cache_depth_decay)*float32(n)
	return pc
}

// Find the chunk. If it doesn't exist, create it.
// This is a speed critical function.
func ChunkFind_WLwWLc(coord chunkdb.CC) *chunk {
	// Need a read lock on the cache, no changes will be done (yet). Assume the chunk is found, which is the normal case.
	worldCacheLock.RLock()
	pc := chunkFindRO(coord)
	worldCacheLock.RUnlock()
	if pc != nil {
		if pc.jellyBlocks != nil {
			// There may be jelly blocks that should be restored
			pc.Lock()
			// It may have changed between the test and the lock, but minimal chance
			pc.RestoreJellyBlocks(false)
			pc.Unlock()
		}
		return pc
	}

	// Now a write lock is needed to save the new chunk in the cache.
	worldCacheLock.Lock()
	// Meanwhile, before the lock was created, the chunk may have been created by another process.
	// It may look wasteful to call chunkFindRO again, but it only happens for new chunks.
	pc = chunkFindRO(coord)
	if pc != nil {
		worldCacheLock.Unlock()
		return pc
	}
	// Didn't find the chunk (again), get it from disk or create one.
	pc = dBFindChunkFromFS(coord)

	AddChunkToHashTable(pc)
	worldCacheLock.Unlock()

	return pc
}

func AddChunkToHashTable(pc *chunk) {
	ind := chunkFindHash(pc.coord)
	pc.next = world_cache[ind]
	world_cache[ind] = pc // Insert the new pointer first
	worldCacheNumChunks++
}

func RemoveChunkFromHashTable(pc *chunk) {
	ind := chunkFindHash(pc.coord)
	for ppc := &world_cache[ind]; *ppc != nil; ppc = &(*ppc).next {
		if *ppc == pc {
			*ppc = pc.next
			pc.next = nil
			worldCacheNumChunks--
			return
		}
	}
}
