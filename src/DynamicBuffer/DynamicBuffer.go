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

//
// A buffer with an initial capacity. Support incremental addition of data
// and extend the capacity when needed. Optionally, the data can be compressed
//
package DynamicBuffer

import (
	"log"
)

// This is a private structure, the factory function is needed to get a buffer
// TODO: It is possible to use two different buffers depending using compression or not,
// which would speed up when there is no need to have conditinal dependency om flags.
// TODO: It may be more effective to re-use the same buffer every time, and return a copy
// with the exact size. However, that would not be safe from multi processes.
type buffer struct {
	buff []byte
	size int
	comp bool // True if the data is compressed
}

func MakeBuffer(capacity int) *buffer {
	return &buffer{make([]byte, capacity), 0, false}
}

func MakeCompressedBuffer(capacity int) *buffer {
	buff := buffer{make([]byte, capacity), 0, true}
	return &buff
}

func (b *buffer) Add(d byte) {
	if b.comp {
		if b.size == 0 {
			// First data.
			b.add(d)
			b.add(1)
		} else if b.buff[b.size-2] == d && b.buff[b.size-1] != 255 {
			// The same, simply increase the count
			b.buff[b.size-1]++
		} else {
			// Either a new value, or the value did not fit (max count 255)
			b.add(d)
			b.add(1)
		}
	} else {
		// No compression, simply add the byte
		b.add(d)
	}
}

func (b *buffer) add(d byte) {
	if b.size == cap(b.buff) {
		// fmt.Printf("DynamicBuffer.Add: Resize from %d to %d\n", b.size, b.size*2)
		b2 := make([]byte, b.size*2)
		copy(b2, b.buff)
		b.buff = b2
	}
	b.buff[b.size] = d
	b.size++
}

// Return the data as a slice
func (b *buffer) Bytes() []byte {
	return b.buff[0:b.size]
}

// Compression is done according to PROT-001.

// Need a helper struct to unpack compressed data
type unpackBuffer struct {
	buff  []byte // This is the buffer we will unpack
	index int    // How far we have come in the unpacking
	count byte   // How many that have been unpacked from that position
}

// Factory that creates the data needed for the state when doing uncompress
func MakeUncompressBuffer(buff []byte) *unpackBuffer {
	b := unpackBuffer{buff, 0, 0}
	return &b
}

// Return true if all bytes have been read from this compressed buffer
func (b *unpackBuffer) IsAtEOF() bool {
	return b.buff[b.index+1] == b.count && b.index+2 == len(b.buff)
}

// Get next unpacked byte, return a flag marking success or failure
func (b *unpackBuffer) GetOne() (byte, bool) {
	if b.index+1 >= len(b.buff) {
		log.Printf("DynamicBuffer::GetOne buffer length %d, cap %d (%#v)\n", len(b.buff), cap(b.buff), *b)
		panic("")
		return 0, false
	}
	// If the last byte has been taken from the latest pair, go on to next pair
	if b.buff[b.index+1] == b.count {
		b.count = 0
		b.index += 2
	}
	b.count++
	return b.buff[b.index], true
}
