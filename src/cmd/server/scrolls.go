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
	"fmt"
	"inventory"
)

const (
	ScrollResurrectionPoint = iota
)

// This type implements the inventory.Object interface.
type Scroll struct {
	Level uint32
	Type  uint8 // The  type
	Count uint8 // The number of this type
}

// Use a scroll. The argument is always a user pointer. Return a function
// that actually does the job.
func (itm *Scroll) Use(p interface{}) func() bool {
	up, ok := p.(*user)
	if !ok || up == nil {
		panic("Not a user pointer")
	}
	if up.pl.Dead {
		return nil
	}
	var f func() bool
	switch itm.Type {
	case ScrollResurrectionPoint:
		f = func() (ret bool) {
			up.Lock()
			up.pl.ReviveSP = up.pl.Coord
			up.pl.Inventory.Remove(itm.ID(), itm.GetLevel())
			up.Unlock()
			return true
		}
	}
	return f
}

func (scroll *Scroll) Classification() inventory.TClassification {
	return inventory.TScroll
}

func (itm *Scroll) AddTest(o inventory.Object) bool {
	p, ok := o.(*Scroll)
	if !ok {
		return false
	}
	if p.Type == itm.Type {
		if itm.Count < 255 {
			itm.Count++
		}
		return true
	}
	return false
}

func (itm *Scroll) String() (ret string) {
	var descr string
	switch itm.Type {
	case ScrollResurrectionPoint:
		descr = "resurrection point"
	}
	if itm.Count == 1 {
		ret = "One scroll of " + descr
	} else {
		ret = fmt.Sprintf("%d scrolls of %s", itm.Count, descr)
	}
	return
}

func (itm *Scroll) ID() (ret string) {
	switch itm.Type {
	case ScrollResurrectionPoint:
		ret = ItemScrollRessID
	}
	return
}

func (itm *Scroll) GetCount() uint8 {
	return itm.Count
}

func (item *Scroll) DecrementCount() uint8 {
	item.Count--
	return item.Count
}

func (itm *Scroll) GetLevel() uint32 {
	return itm.Level
}

func (itm *Scroll) Value(playerLevel uint32) float32 {
	// The resurrection scrolls all work the same, regardless of the level. But give lower value if they were
	// found in a low level area, to discourage easy farming
	return ItemValueAsDrop(playerLevel, itm.Level, 0)
}

func MakeResurrectionPointScroll(level uint32) inventory.Object {
	return &Scroll{Type: ScrollResurrectionPoint, Count: 1, Level: level}
}
