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

import (
	// "fmt"
	"chunkdb"
	"github.com/larspensjo/Go-simplex-noise/simplexnoise"
	"math"
	"time"
)

func dBdensity(xf, yf, zf float64) float64 {
	return simplexnoise.Noise3(xf*0.01, yf*0.01, zf*0.01)/2 + 0.5 // Now in range 0-1
}

var DBCreateStats struct {
	Num     int
	TotTime time.Duration
}

// For a chunk at a coordinate, create it.
// There is no need for a lock, as no one can access the chunk
// TODO: Some algorithms depend on looking at the block above, which can only be done when inside the
// chunk. If the test would require looking at another chunk, the tets skipped. This leads to some special
// effects failure.
func dBCreateChunk(c chunkdb.CC) *chunk {
	start := time.Now()
	ch := new(chunk)
	ch.rc = new(raw_chunk)
	ch.coord = c
	z1 := int(c.Z * CHUNK_SIZE)

	for x := int32(0); x < CHUNK_SIZE; x++ {
		xf := float64(x + c.X*CHUNK_SIZE)
		for y := int32(0); y < CHUNK_SIZE; y++ {
			yf := float64(y + c.Y*CHUNK_SIZE)
			highFreq := 20 * simplexnoise.Noise2(xf*0.016, yf*0.016)  // This will generate high frequency terrain
			f := simplexnoise.Noise2(xf*0.0025, yf*0.0025)            // Factor to modulate the high frequency amplitude
			lowFreq := 15 * simplexnoise.Noise2(xf*0.0013, yf*0.0013) // Low frequency terrain
			stoneheight := math.Floor(2.5 + highFreq*f*f + lowFreq)

			// Given 'height', fill in the content of the current chunk. Iterate from high 'z' to low,
			// to enable tests that depends on the block above.
			for z := CHUNK_SIZE - 1; z >= 0; z-- {
				if *inhibitCreateChunks {
					ch.rc[x][y][z] = BT_Air
					continue
				}
				zf := float64(z + z1)
				if zf > FLOATING_ISLANDS_LIM {
					// Use a gradial transient, or all islands would have a hard cut off.
					f := (1-FLOATING_ISLANDS_PROB)/CHUNK_SIZE*(zf-FLOATING_ISLANDS_LIM) + FLOATING_ISLANDS_PROB
					if f > 1 {
						f = 1
					}
					density := f * dBdensity(xf/2, yf/2, zf) // Use a compressed layout in height
					if density > FLOATING_ISLANDS_PROB {
						if z != CHUNK_SIZE-1 && blockIsInvisible[ch.rc[x][y][z+1]] {
							ch.rc[x][y][z] = BT_Soil // Put grass on top
						} else {
							ch.rc[x][y][z] = BT_Stone
						}
					} else {
						ch.rc[x][y][z] = BT_Air
					}
					continue
				}
				density := dBdensity(xf/2, yf/2, zf)
				soildepth := math.Floor(2*simplexnoise.Noise2(xf*0.012, yf*0.012) + 2.8)
				if stoneheight > WORLD_SOIL_LEVEL {
					soildepth = 0
				} else if soildepth+stoneheight > WORLD_SOIL_LEVEL {
					soildepth = WORLD_SOIL_LEVEL - stoneheight
				}
				height := stoneheight + soildepth

				ch.rc[x][y][z] = BT_Air
				if zf <= stoneheight {
					// Initialize with stone, may be updated below
					if zf > 24 {
						ch.rc[x][y][z] = BT_Snow
					} else {
						ch.rc[x][y][z] = BT_Stone
					}
				} else if zf <= stoneheight+soildepth {
					// Initialize with soil, may be updated below
					ch.rc[x][y][z] = BT_Soil
				}

				// Excavate some holes in the terrain. Don't let the hole go too deep
				const HOLEDEPTH = 50 // Max depth of hole
				if zf > -HOLEDEPTH && zf < HOLEDEPTH {
					density := dBdensity(xf, yf, zf) // This costs a lot of CPU
					fadeoff := 1.0
					if zf >= -HOLEDEPTH && zf <= 0 {
						fadeoff = (HOLEDEPTH + zf) / HOLEDEPTH
					} else if zf < -HOLEDEPTH {
						fadeoff = 0
					}
					if density*fadeoff > 0.7 {
						ch.rc[x][y][z] = BT_Air
					}
				}

				// Some special cases if below water line
				if zf <= 0 {
					if ch.rc[x][y][z] == BT_Air {
						ch.rc[x][y][z] = BT_Water
					} else if ch.rc[x][y][z] == BT_Soil {
						ch.rc[x][y][z] = BT_Stone
					}
					if ch.rc[x][y][z] == BT_Stone && z+z1 == 0 && blockIsInvisible[ch.rc[x][y][1]] {
						// Replace stone with sand if it is at water level and air above.
						ch.rc[x][y][0] = BT_Sand
					}
				}

				a := density > 0.5-CnfgCaveWidth/2 && density < 0.5+CnfgCaveWidth/2
				if zf <= height && a {
					density2 := dBdensity(1000-xf/2, 1000-yf/2, 1000-zf) // Use a compressed layout in height
					b := density2 > 0.5-CnfgCaveWidth/2 && density2 < 0.5+CnfgCaveWidth/2
					if b && ch.rc[x][y][z] != BT_Water {
						ch.rc[x][y][z] = BT_Air
					}
				}

				// Add some scenery
				if ch.rc[x][y][z] == BT_Soil && z+1 < CHUNK_SIZE && blockIsInvisible[ch.rc[x][y][z+1]] {
					// This is a candidate for a tree override
					const (
						t3      = 0.0005 // Very few big trees
						t2      = 0.005
						t1      = 0.010
						tflower = 0.012 // Less flowers than tuft of grass
						ttuft   = 0.020
					)
					rnd := math.Abs(simplexnoise.Noise2(xf*422.34, yf*234.123)) // Without scaling, there is a line where xf+yf==0 gives rnd=0
					if rnd > t1 {
						continue // Not needed for the algorithm but will save a call to Noise2.
					}
					// Use a low frequency function to make less trees for some areas.
					lowFreq := 1 - math.Abs(simplexnoise.Noise2(xf*0.002, yf*0.002))
					// The lowFreq function takes away too many trees, ease it up a little
					lowFreq = 1 - lowFreq*lowFreq
					// fmt.Printf("%.5f ", lowFreq)
					switch {
					case rnd < t3*lowFreq:
						ch.rc[x][y][z+1] = BT_Tree3
					case rnd < t2*lowFreq:
						ch.rc[x][y][z+1] = BT_Tree2
					case rnd < t1*lowFreq:
						ch.rc[x][y][z+1] = BT_Tree1
					case rnd < tflower*lowFreq:
						ch.rc[x][y][z+1] = BT_Flowers
					case rnd < ttuft*lowFreq:
						ch.rc[x][y][z+1] = BT_Tuft
					}
				}
			}
		}
	}
	ch.compress()
	ch.updateChecksum()
	ch.touched = true
	delta := time.Now().Sub(start)
	DBCreateStats.Num++
	DBCreateStats.TotTime += delta
	return ch
}
