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

package twof

import (
	"fmt"
	"math"
)

type TwoF [2]float64

func (tf *TwoF) String() string {
	return fmt.Sprintf("(%.2f,%.2f)", tf[0], tf[1])
}

// Compute the distance from one coordinate to another
func (p1 *TwoF) Dist(p2 *TwoF) float64 {
	dx, dy := p2[0]-p1[0], p2[1]-p1[1]
	return math.Sqrt(dx*dx + dy*dy)
}
