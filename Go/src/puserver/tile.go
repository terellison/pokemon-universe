/*Pokemon Universe MMORPG
Copyright (C) 2010 the Pokemon Universe Authors

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.*/
package main

import (
	list "container/list"
	pos "putools/pos"
	pul "pulogic"
)

const (
	TILEBLOCK_BLOCK       int = 1
	TILEBLOCK_WALK            = 2
	TILEBLOCK_SURF            = 3
	TILEBLOCK_TOP             = 4
	TILEBLOCK_BOTTOM          = 5
	TILEBLOCK_RIGHT           = 6
	TILEBLOCK_LEFT            = 7
	TILEBLOCK_TOPRIGHT        = 8
	TILEBLOCK_BOTTOMRIGHT     = 9
	TILEBLOCK_BOTTOMLEFT      = 10
	TILEBLOCK_TOPLEFT         = 11
)

type Tile struct {
	Position 	pos.Position
	Blocking 	int
	Location 	*Location

	Layers    	pul.LayerMap
	Creatures 	pul.CreatureMap // List of creatures who are active on this tile
	Events    	*list.List
}

// NewTile creates a Tile object with Position as parameter
func NewTile(_pos pos.Position) *Tile {
	t := &Tile{Position: _pos}
	t.Blocking = TILEBLOCK_WALK
	t.Layers = make(pul.LayerMap)
	t.Creatures = make(pul.CreatureMap)
	t.Location = nil
	t.Events = list.New()

	return t
}

// NewTileExt creates a Position from _x, _y, _z and then calls NewTile to create a new Tile object
func NewTileExt(_x int, _y int, _z int) *Tile {
	return NewTile(pos.NewPositionFrom(_x, _y, _z))
}

// AddLayer adds a new TileLayer to the tile. 
// If the layer already exists it will return that one otherwise it'll make a new one
func (t *Tile) AddLayer(_layer int, _sprite int) (layer *pul.TileLayer) {
	layer = t.GetLayer(_layer)
	if layer == nil {
		layer = &pul.TileLayer{ Layer: _layer, SpriteID: _sprite }
		t.Layers[_layer] = layer
	}

	return
}

func (t *Tile) AddEvent(_event ITileEvent) {
	t.Events.PushBack(_event)
}

// GetLayer returns a TileLayer object if the layer exists, otherwise nil
func (t *Tile) GetLayer(_layer int) *pul.TileLayer {
	if layer, ok := t.Layers[_layer]; !ok {
		return layer
	}

	return nil
}

func (t *Tile) GetPosition() pos.Position {
	return t.Position
}

func (t *Tile) GetBlocking() int {
	return t.Blocking
}

func (t *Tile) GetCreatures() pul.CreatureMap {
	return t.Creatures
}

func (t *Tile) GetLayers() pul.LayerMap {
	return t.Layers
}

func (t *Tile) GetEvents() *list.List {
	return t.Events
}

func (t *Tile) GetLocation() pul.ILocation {
	return t.Location
}

// CheckMovement checks if a creature can move to this tile
func (t *Tile) CheckMovement(_creature pul.ICreature, _dir int) int {
	movement := _creature.GetMovement()
	blocking := t.Blocking

	if blocking != TILEBLOCK_WALK {
		if blocking == TILEBLOCK_BLOCK ||
			(blocking == TILEBLOCK_SURF && movement != MOVEMENT_SURF) ||
			(blocking == TILEBLOCK_TOP && _dir == DIR_SOUTH) ||
			(blocking == TILEBLOCK_BOTTOM && _dir == DIR_NORTH) ||
			(blocking == TILEBLOCK_LEFT && _dir == DIR_EAST) ||
			(blocking == TILEBLOCK_RIGHT && _dir == DIR_WEST) ||
			(blocking == TILEBLOCK_TOPLEFT && (_dir == DIR_EAST || _dir == DIR_SOUTH)) ||
			(blocking == TILEBLOCK_TOPRIGHT && (_dir == DIR_WEST || _dir == DIR_SOUTH)) ||
			(blocking == TILEBLOCK_BOTTOMLEFT && (_dir == DIR_EAST || _dir == DIR_NORTH)) ||
			(blocking == TILEBLOCK_BOTTOMRIGHT && (_dir == DIR_WEST || _dir == DIR_NORTH)) {
			return RET_NOTPOSSIBLE
		}
	}

	return RET_NOERROR
}

// AddCreature adds a new active creature to this tile
func (t *Tile) AddCreature(_creature pul.ICreature, _checkEvents bool) (ret int) {
	ret = RET_NOERROR

	if _checkEvents && t.Events.Len() > 0 {
		var i int = 0
		for e := t.Events.Front(); e != nil; e = e.Next() {
			event, valid := e.Value.(ITileEvent)
			if valid {
				ret = event.OnCreatureEnter(_creature, ret)
			}

			if ret == RET_NOTPOSSIBLE {
				return
			}
			i++
		}
	}

	_, found := t.Creatures[_creature.GetUID()]
	if !found {
		t.Creatures[_creature.GetUID()] = _creature
	}

	return
}

// RemoveCreature removes an active creature from this tile
func (t *Tile) RemoveCreature(_creature pul.ICreature, _checkEvents bool) (ret int) {
	ret = RET_NOERROR

	if _checkEvents && t.Events.Len() > 0 {
		for e := t.Events.Front(); e != nil; e = e.Next() {
			event, valid := e.Value.(ITileEvent)
			if valid {
				ret = event.OnCreatureLeave(_creature, ret)
			}

			if ret == RET_NOTPOSSIBLE {
				return
			}
		}
	}

	delete(t.Creatures, _creature.GetUID())

	return
}
