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
// Collection for functions that pack and unpack data from byte vectors.
//

import (
// "fmt"
)

func ParseUint64(b []byte) (uint64, []byte, bool) {
	if len(b) < 8 {
		return 0, nil, false
	}
	var res uint64
	for i := 0; i < 8; i++ {
		res |= uint64(b[i]) << (uint(i) * 8)
	}
	return res, b[8:], true
}

func ParseInt32(b []byte) (int32, []byte, bool) {
	if len(b) < 4 {
		return 0, nil, false
	}
	var res uint32
	for i := 0; i < 4; i++ {
		res |= uint32(b[i]) << (uint(i) * 8)
	}
	return int32(res), b[4:], true
}

func ParseUint32(b []byte) (uint32, []byte, bool) {
	if len(b) < 4 {
		return 0, nil, false
	}
	var res uint32
	for i := 0; i < 4; i++ {
		res |= uint32(b[i]) << (uint(i) * 8)
	}
	return res, b[4:], true
}

func ParseUint16(b []byte) (uint16, []byte, bool) {
	if len(b) < 2 {
		return 0, nil, false
	}
	var res uint16
	for i := 0; i < 2; i++ {
		res |= uint16(b[i]) << (uint(i) * 8)
	}
	return res, b[2:], true
}

func EncodeUint16(n uint16, b []byte) {
	if cap(b) < 2 {
		panic("EncodeUint16 too small buffer")
	}
	b[0] = byte(n & 0xFF)
	b[1] = byte((n >> 8) & 0xFF)
}

func EncodeUint32(n uint32, b []byte) {
	if cap(b) < 4 {
		panic("EncodeUint32 too small buffer")
	}
	b[0] = byte(n & 0xFF)
	b[1] = byte((n >> 8) & 0xFF)
	b[2] = byte((n >> 16) & 0xFF)
	b[3] = byte((n >> 24) & 0xFF)
}

func EncodeUint64(n uint64, b []byte) {
	if cap(b) < 8 {
		panic("EncodeUint32 too small buffer")
	}
	b[0] = byte(n & 0xFF)
	b[1] = byte((n >> 8) & 0xFF)
	b[2] = byte((n >> 16) & 0xFF)
	b[3] = byte((n >> 24) & 0xFF)
	b[4] = byte((n >> 32) & 0xFF)
	b[5] = byte((n >> 40) & 0xFF)
	b[6] = byte((n >> 48) & 0xFF)
	b[7] = byte((n >> 56) & 0xFF)
}
