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

package quadtree

//
// This package is used for keeping track of what objects are close to each other.
// The cost of checking all possible objects would grow with the square of the number of
// objects, so this package will recursively divide a volume into 2 (in every dimension, giving 8
// sub cubes) when the number of objects exceeds a certain limit.
//

import (
	"fmt"
	"log"
	"sync"
	. "twof"
)

//
// Depth estimate:
// 10000 players
// Divided into 4^6 squares gives 2 players per square.
// Allow for higher concentration at some places, and it should still be enough.

const (
	maxQuadtreeDepth      = 6   // Do not make more levels below this
	minObjectsPerQuadtree = 5   // Lower limit of the number of objects before collapsing
	maxObjectsPerQuadtree = 10  // Upper limit of the number of objects before expanding
	expandFactor          = 1.3 // How much the are is expanded when the volume is too small
)

// The objects that are managed must fulfill this interface
type Object interface {
	GetPreviousPos() *TwoF // The previous coordnate
	GetType() uint8        // Not needed by QuadTree
	GetZ() float64         // Not needed by QuadTree
	GetId() uint32         // Used by the client to identify objects.
	GetDir() float32       // Get looking direction in radians
}

// Use MakeQuadtree() to get one!
type Quadtree struct {
	corner1     TwoF // Lower left corner
	corner2     TwoF // upper right corner
	center      TwoF // middle of the square
	children    [2][2]*Quadtree
	hasChildren bool
	depth       int
	numObjects  int // Sum of all objects in all children
	objects     []Object
	sync.RWMutex
}

// Remember current Quadtree, for debugging purposes
var debugCurrent *Quadtree

// Check if the Quadtree is big enough to contain the given position. This is done by simply making
// a bigger initial cube and moving all objects to the new one. Not a very cheap solution, but
// it is expected to be done rarely.
// Return the verified Quadtree.
// TODO: This doesn't work with the current locking strategy.
func (t *Quadtree) checkExpand(tf *TwoF) {
	changed := false
	newCorner1 := t.corner1
	newCorner2 := t.corner2
	for i := 0; i < 2; i++ {
		if tf[i] < t.corner1[i] {
			changed = true
			newCorner1[i] = t.corner2[i] - (t.corner2[i]-tf[i])*expandFactor
		}
		if tf[i] > t.corner2[i] {
			changed = true
			newCorner2[i] = t.corner1[i] + (tf[i]-t.corner1[i])*expandFactor
		}
	}
	if !changed {
		return
	}
	// log.Printf("Quadtree: Expanding (%.0f-%.0f) to (%.0f-%.0f)\n", t.corner1, t.corner2, newCorner1, newCorner2)
	t.destroyChildren() // This will move all objects to the root.
	t.corner1 = newCorner1
	t.corner2 = newCorner2
	// Next time an object is added, the tree will expand again.
}

// Return true if this Quadtree is empty. Used for debugging and testing.
func (t *Quadtree) Empty() bool {
	// No need to lock for this operation.
	// fmt.Printf("Quadtree::Empty %d/%d objects\n", t.numObjects, len(t.objects))
	return t.numObjects == 0 && !t.hasChildren && len(t.objects) == 0
}

func (t *Quadtree) Stats_RLq() string {
	t.RLock()
	defer t.RUnlock()
	return t.string2(false)
}

func (t *Quadtree) String_RLq() string {
	t.RLock()
	defer t.RUnlock()
	return t.string2(true)
}

func (t *Quadtree) string2(verbose bool) string {
	// Must not call the recursive implicit String() conversion, which would hand from read lock.
	var str string
	if verbose && t.objects != nil {
		str += fmt.Sprintf("%*sobjs:", t.depth-1, "")
		for _, o := range t.objects {
			str += fmt.Sprintf("\n%*s%v ", t.depth-1, "", o)
		}
	}
	if t.hasChildren {
		for x := 0; x < 2; x++ {
			for y := 0; y < 2; y++ {
				if verbose {
					str += fmt.Sprintf("%*sChild [%d,%d]:\n%v", t.depth-1, "", x, y, t.children[x][y].string2(verbose))
				} else {
					str += t.children[x][y].string2(verbose)
				}
			}
		}
	}
	ret := ""
	if verbose || t.numObjects > 0 {
		ret = fmt.Sprintf("%*scorner1: ", t.depth-1, "") + t.corner1.String() +
			" corner2: " + t.corner2.String() +
			fmt.Sprintf("\n%*shasChildren: %v, depth: %d, numObjects: %d\n",
				t.depth-1, "", t.hasChildren, t.depth, t.numObjects) + str
	}
	return ret
}

func MakeQuadtree(c1, c2 TwoF, depth int) *Quadtree {
	var t Quadtree
	t.corner1 = c1
	t.corner2 = c2
	t.center = TwoF{(c1[0] + c2[0]) / 2, (c1[1] + c2[1]) / 2}
	t.depth = depth
	return &t
}

// Adds or removes an object from the children. The size of objects are considered to be 0,
// which means an object can only be located in one child.
func (t *Quadtree) fileObject(o Object, c *TwoF, add bool) {
	// Figure out in which child(ren) the object belongs
	for x := 0; x < 2; x++ {
		if x == 0 {
			if c[0] > t.center[0] {
				continue
			}
		} else if c[0] < t.center[0] {
			continue
		}

		for y := 0; y < 2; y++ {
			if y == 0 {
				if c[1] > t.center[1] {
					continue
				}
			} else if c[1] < t.center[1] {
				continue
			}

			//Add or remove the object
			if add {
				t.children[x][y].add(o, c)
			} else {
				t.children[x][y].remove(o, c)
			}
			return
		}
	}
}

// Take a leaf in the Quadtree, add children, and move all objects to the children.
func (t *Quadtree) makeChildren() {
	for x := 0; x < 2; x++ {
		var minX, maxX float64
		if x == 0 {
			minX = t.corner1[0]
			maxX = t.center[0]
		} else {
			minX = t.center[0]
			maxX = t.corner2[0]
		}

		for y := 0; y < 2; y++ {
			var minY, maxY float64
			if y == 0 {
				minY = t.corner1[1]
				maxY = t.center[1]
			} else {
				minY = t.center[1]
				maxY = t.corner2[1]
			}

			t.children[x][y] = MakeQuadtree(TwoF{minX, minY},
				TwoF{maxX, maxY},
				t.depth+1)
		}
	}

	// Add all objects to the new children and remove them from "objects"
	for _, it := range t.objects {
		// fmt.Printf("%*smakeChildren move %v from (%v-%v)\n", t.depth-1, "", it, t.corner1, t.corner2)
		t.fileObject(it, it.GetPreviousPos(), true) // Use previous pos as the object may be moving asynchronously
	}
	t.objects = nil
	t.hasChildren = true
}

// Destroys the children of this, and moves all objects in its descendants
// to the "objects" set
func (t *Quadtree) destroyChildren() {
	// fmt.Printf("%*sDestroyChildren (%v-%v) numobjs %d\n", t.depth-1, "", t.corner1, t.corner2, t.numObjects)
	//Move all objects in descendants of this to the "objects" set
	t.collectObjects(&t.objects)

	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			t.children[x][y] = nil
		}
	}

	t.hasChildren = false
}

// Removes the specified object at the indicated position.
func (t *Quadtree) remove(o Object, pos *TwoF) {
	t.numObjects--
	if t.numObjects < 0 {
		log.Println(">>>>Quadtree:remove numobjects < 0")
		log.Printf(">>>>Pos %v, Current tree %p\n>>>>Object %#v\n>>>>Quadtree %s\n", pos, debugCurrent, o, t.string2(true))
		log.Println(">>>>Actual position in tree:", debugCurrent.searchForObject(o).string2(true))
		log.Panicln("Quadtree:remove numobjects < 0")
	}

	if t.hasChildren && t.numObjects < minObjectsPerQuadtree {
		t.destroyChildren()
	}

	if t.hasChildren {
		t.fileObject(o, pos, false)
	} else {
		// Find o in the local list
		// fmt.Printf("%*sremove: %v (%p) from %v\n", t.depth-1, "", o, o, t.objects)
		for i, o2 := range t.objects {
			if o2 == o {
				// Found it
				if last := len(t.objects) - 1; i == last {
					t.objects = t.objects[:last]
				} else {
					// Move the last element to this position
					t.objects[i] = t.objects[last]
					t.objects = t.objects[:last]
				}
				return
			}
		}
		log.Println(">>>>Quadtree:remove failed to find object")
		log.Printf(">>>>Pos %v, Current tree %p\n>>>>Object %#v\n>>>>Quadtree %s\n", pos, debugCurrent, o, t.string2(true))
		log.Println(">>>>Actual position in tree:", debugCurrent.searchForObject(o).string2(true))
		log.Panicln("Quadtree:remove failed to find object")
	}
}

//Removes the specified object at the indicated position. We can't ask
func (t *Quadtree) Remove_WLq(o Object) {
	debugCurrent = t
	t.Lock()
	t.remove(o, o.GetPreviousPos())
	t.Unlock()
}

// Add an object.
func (t *Quadtree) Add_WLq(o Object, pos *TwoF) {
	// fmt.Printf("%*sAdd: %v to (%v-%v)\n", t.depth-1, "", o, t.corner1, t.corner2)
	t.Lock()
	t.checkExpand(pos)
	t.add(o, pos)
	t.Unlock()
}

// Add an object
func (t *Quadtree) add(o Object, c *TwoF) {
	// fmt.Printf("%*sAdd: %v to (%v-%v)\n", t.depth-1, "", o, t.corner1, t.corner2)
	t.numObjects++
	if !t.hasChildren && t.depth < maxQuadtreeDepth && t.numObjects > maxObjectsPerQuadtree {
		t.makeChildren()
	}

	if t.hasChildren {
		t.fileObject(o, c, true) // Use previous pos as the object may be moving asynchronously
	} else {
		t.objects = append(t.objects, o)
	}
}

// Test that an object, at the specified position, is in the quadtree where it should be.
func (t *Quadtree) testPresent(o Object, pos *TwoF) bool {
	if !t.hasChildren {
		// There are no children to this tree, which means the object should be in the list of objects.
		for _, o2 := range t.objects {
			if o2 == o {
				// Found it
				return true
			}
		}
		return false
	}
	// Figure out in which child(ren) the object belongs
	for x := 0; x < 2; x++ {
		if x == 0 {
			if pos[0] > t.center[0] {
				continue
			}
		} else if pos[0] < t.center[0] {
			continue
		}

		for y := 0; y < 2; y++ {
			if y == 0 {
				if pos[1] > t.center[1] {
					continue
				}
			} else if pos[1] < t.center[1] {
				continue
			}

			return t.children[x][y].testPresent(o, pos)
		}
	}
	// This shall never happen!
	log.Panicln("Quadtree.testPresent failed", o, pos, t, debugCurrent)
	return false
}

// Changes the position of an object in this from oldPos to object.pos
func (t *Quadtree) Move_WLq(o Object, to *TwoF) {
	debugCurrent = t
	from := o.GetPreviousPos()
	// Assume the obect was moved to another part of the quadtree
	changed := true
	// Usually, the object will not be moved from one part of the quadtree to another. Do a test if that is
	// the case, in which case only a read lock will be needed. This will add a constant cost, but will
	// allow many more parallel threads.
	t.RLock()
	if t.testPresent(o, to) {
		changed = false
	}
	t.RUnlock()
	if changed {
		t.Lock()
		t.remove(o, from)
		t.checkExpand(to)
		t.add(o, to)
		t.Unlock()
	}
}

// Adds all objects in this or its descendants to the specified set
// TODO: The same objects can be added several times to the set
func (t *Quadtree) collectObjects(os *[]Object) {
	if t.hasChildren {
		for x := 0; x < 2; x++ {
			for y := 0; y < 2; y++ {
				t.children[x][y].collectObjects(os)
			}
		}
	} else {
		// Add all "objects" into the provided list, if they are not already there
		for _, o := range t.objects {
			found := false
			for _, o2 := range *os {
				if o2 == o {
					found = true
					break
				}
			}
			if !found {
				*os = append(*os, o)
			}
		}
	}
	// fmt.Printf("%*scollectObjects (%v-%v) result: %v\n", t.depth-1, "", t.corner1, t.corner2, os)
}

// Find all objects within radius "dist" from "pos".
func (t *Quadtree) findNearObjects(pos *TwoF, dist float64, objList *[]Object) {
	if !t.hasChildren {
		for _, o := range t.objects {
			// TODO: Use a squared distance instead, to save time for doing sqrt.
			if pos.Dist(o.GetPreviousPos()) > dist {
				continue // This object was too far away
			}
			*objList = append(*objList, o)
		}
	} else {
		// Traverse all sub squares that are inside the distance. More than one can match.
		for x := 0; x < 2; x++ {
			if x == 0 {
				if pos[0]-dist > t.center[0] {
					continue
				}
			} else if pos[0]+dist < t.center[0] {
				continue
			}
			for y := 0; y < 2; y++ {
				if y == 0 {
					if pos[1]-dist > t.center[1] {
						continue
					}
				} else if pos[1]+dist < t.center[1] {
					continue
				}
				t.children[x][y].findNearObjects(pos, dist, objList)
			}
		}
	}
}

// Find all objects within radius "dist" from "pos", excluding duplicates
func (t *Quadtree) FindNearObjects_RLq(pos *TwoF, dist float64) []Object {
	var objList []Object
	t.RLock()
	t.findNearObjects(pos, dist, &objList)
	t.RUnlock()
	return objList
}

// Do full tree search for an object, not based on position. Used for debugging purposes.
func (this *Quadtree) searchForObject(obj Object) *Quadtree {
	if this == nil {
		return nil
	}
	if !this.hasChildren {
		for _, o := range this.objects {
			if o == obj {
				return this
			}
		}
	} else {
		for x := 0; x < 2; x++ {
			for y := 0; y < 2; y++ {
				ret := this.children[x][y].searchForObject(obj)
				if ret != nil {
					return ret
				}
			}
		}
	}
	return nil
}
