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
	"fmt"
	"strings"
	"sync"
	"time"

	pnet "network" // PU Network package
	pos "position" // Position package
)

type GameState int

const (
	GAME_STATE_STARTUP GameState = iota
	GAME_STATE_INIT
	GAME_STATE_NORMAL
	GAME_STATE_CLOSED
	GAME_STATE_CLOSING
)

type Game struct {
	State         		GameState
	
	Creatures     		CreatureList
	Players       		PlayerList
	PlayersDiscon		PlayerList

	Chat				*Chat
	Locations			*LocationStore

	mutexCreatureList   *sync.RWMutex
	mutexPlayerList     *sync.RWMutex
	mutexDisconnectList *sync.RWMutex
}

func NewGame() *Game {
	game := Game{}
	game.State = GAME_STATE_STARTUP
	
	// Initialize maps
	game.Creatures = make(CreatureList)
	game.Players = make(PlayerList)
	game.PlayersDiscon = make(PlayerList)
	
	game.Chat = NewChat()

	// Mutexes
	game.mutexCreatureList = new(sync.RWMutex)
	game.mutexPlayerList = new(sync.RWMutex)
	game.mutexDisconnectList = new(sync.RWMutex)

	return &game
}

func (the *Game) Load() (LostIt bool) {
	LostIt = true // fuck >:(
	g_map = NewMap()
	the.Locations = NewLocationStore()

	g_logger.Println(" - Loading locations")
	// Load locations
	if err := the.Locations.Load(); err != nil {
		g_logger.Println(err)
		LostIt = false
	}

	// Load worldmap
	g_logger.Println(" - Loading worldmap")
	start := time.Now().UnixNano()
	if err := g_map.Load(); err != nil {
		g_logger.Println(err)
		LostIt = false
	} else {
		g_logger.Printf(" - Map loaded in %dms\n", (time.Now().UnixNano()-start)/1e6)
	}

	return
}

func (g *Game) CheckCreatures() {
	// TODO: Change this to a scheduler
	go func() {
		time.Sleep(1e9)
		g.CheckCreatures()
	}()
	
	g.mutexCreatureList.RLock()
	defer g.mutexCreatureList.RUnlock()
	
	for _, creature := range g.Creatures {
		creature.OnThink(1000)
	}
}

func (g *Game) GetPlayerByGuid(_guid uint64) (*Player, bool) {
	v, ok := g.Players[_guid]
	return v, ok
}

func (g *Game) GetPlayerByName(_name string) (*Player, bool) {
	for _, value := range g.Players {
		if value.GetName() == _name {
			return value, true
		}
	}

	return nil, false
}

func (g *Game) OnPlayerLoseConnection(_player *Player) {
	_player.Conn = nil
	_player.TimeoutCounter = 0
	g.PlayersDiscon[_player.GetUID()] = _player
}

func (g *Game) AddCreature(_creature ICreature) {
	// TODO: Maybe only take the creatues from the area the new creature is in. This saves some extra iterating
	// TODO 2: Upgrade this to parallel stuff
	
	fmt.Printf("[%v] Login on (%v,%v)\n", _creature.GetName(), _creature.GetPosition().X, _creature.GetPosition().Y)

	for _, value := range g.Creatures {
		value.OnCreatureAppear(_creature, true)
	}

	g.mutexCreatureList.Lock()
	defer g.mutexCreatureList.Unlock()
	g.Creatures[_creature.GetUID()] = _creature

	if _creature.GetType() == CTYPE_PLAYER {
		g.mutexPlayerList.Lock()
		defer g.mutexPlayerList.Unlock()
		g.Players[_creature.GetUID()] = _creature.(*Player)
		
		// Join default channels
		g.Chat.AddUserToChannel(_creature.(*Player), pnet.CHANNEL_WORLD)
		g.Chat.AddUserToChannel(_creature.(*Player), pnet.CHANNEL_TRADE)
	}
}

func (g *Game) RemoveCreature(_guid uint64) {
	object, exists := g.Creatures[_guid]
	if exists {
		g.mutexCreatureList.Lock()
		defer g.mutexCreatureList.Unlock()

		delete(g.Creatures, _guid)

		if object.GetType() == CTYPE_PLAYER {
			// Join default channels
			g.Chat.RemoveUserFromAllChannels(object.(*Player))
		
			g.mutexPlayerList.Lock()
			defer g.mutexPlayerList.Unlock()
			g_logger.Printf("[Logout] %d - %v logged out\n", object.GetUID(), object.GetName())
			delete(g.Players, _guid)
		}
	}
}

func (g *Game) OnPlayerMove(_creature ICreature, _direction int, _sendMap bool) {
	ret := g.OnCreatureMove(_creature, _direction)

	player := _creature.(*Player)
	if ret == RET_NOTPOSSIBLE {
		player.sendCreatureMove(_creature, _creature.GetTile(), _creature.GetTile())
	} else if ret == RET_PLAYERISTELEPORTED {
		println("Player teleported - START")
		player.sendPlayerWarp()
		player.sendMapData(DIR_NULL)
		println("Player teleported - END")
	} else {
		player.sendMapData(_direction)
	}
}

func (g *Game) OnPlayerTurn(_creature ICreature, _direction int) {
	if _creature.GetDirection() != _direction {
		g.OnCreatureTurn(_creature, _direction)
	}
}

func (g *Game) OnPlayerSay(_creature *Player, _channelId int, _speakType int, _receiver string, _message string) bool {
	toReturn := false
	switch _speakType {
		case pnet.SPEAK_NORMAL:
			toReturn = g.internalCreatureSay(_creature, pnet.SPEAK_NORMAL, _message)
		case pnet.SPEAK_YELL:
			toReturn = g.playerYell(_creature, _message)
		case pnet.SPEAK_WHISPER:
			toReturn = g.playerWhisper(_creature, _message)
		case pnet.SPEAK_PRIVATE:
			toReturn = g.playerSpeakTo(_creature, _speakType, _receiver, _message)
		case pnet.SPEAK_CHANNEL:
			if g.playerTalkToChannel(_creature, _speakType, _message, _channelId) {
				toReturn = true
			} else if _channelId != 0 {
				// Resend in default channel
				toReturn = g.OnPlayerSay(_creature, 0, pnet.SPEAK_NORMAL, _receiver, _message)
			}
		case pnet.SPEAK_BROADCAST:
			toReturn = g.internalBroadcastMessage(_creature, _message)
	}
	
	return toReturn
}

func (g *Game) OnCreatureMove(_creature ICreature, _direction int) (ret ReturnValue) {
	ret = RET_NOTPOSSIBLE

	if !CreatureCanMove(_creature) {
		return
	}

	currentTile := _creature.GetTile()
	destinationPosition := currentTile.Position

	switch _direction {
	case DIR_NORTH:
		destinationPosition.Y -= 1
	case DIR_SOUTH:
		destinationPosition.Y += 1
	case DIR_WEST:
		destinationPosition.X -= 1
	case DIR_EAST:
		destinationPosition.X += 1
	}

	// Check if destination tile exists
	destinationTile, ok := g_map.GetTileFromPosition(destinationPosition)
	if !ok {
		return
	}

	// Check if we can move to the destination tile
	if ret = destinationTile.CheckMovement(_creature, _direction); ret == RET_NOTPOSSIBLE {
		return
	}

	// Update position
	_creature.SetTile(destinationTile)
	
	fmt.Printf("[%v] From (%v,%v) To (%v,%v)\n", _creature.GetName(), currentTile.Position.X, currentTile.Position.Y, destinationPosition.X, destinationPosition.Y)

	// Tell creatures this creature has moved
	g.mutexCreatureList.RLock()
	defer g.mutexCreatureList.RUnlock()
	for _, value := range g.Creatures {
		if value != nil {
			value.OnCreatureMove(_creature, currentTile, destinationTile, false)
		}
	}

	// Move creature object to destination tile
	if ret = currentTile.RemoveCreature(_creature, true); ret == RET_NOTPOSSIBLE {
		return
	}
	if ret = destinationTile.AddCreature(_creature, true); ret == RET_NOTPOSSIBLE {
		currentTile.AddCreature(_creature, false) // Something went wrong, put creature back on old tile
		return
	}

	// If ICreature is a player type we can check for wild encounter
	g.checkForWildEncounter(_creature)

	return
}

func (g *Game) OnCreatureTurn(_creature ICreature, _direction int) {
	if _creature.GetDirection() != _direction {
		_creature.SetDirection(_direction)

		visibleCreatures := _creature.GetVisibleCreatures()
		for _, value := range visibleCreatures {
			value.OnCreatureTurn(_creature)
		}
	}
}

func (g *Game) internalCreatureTeleport(_creature ICreature, _from *Tile, _to *Tile) (ret ReturnValue) {
	ret = RET_PLAYERISTELEPORTED

	println("internalCreatureTeleport - START");
	if _from == nil || _to == nil {
		ret = RET_NOTPOSSIBLE
	} else {
		// Move creature object to destination tile
		fmt.Printf("internalCreatureTeleport - RemoveCreature - %v\n", _from.Position.String())
		if ret = _from.RemoveCreature(_creature, true); ret == RET_NOTPOSSIBLE {
			println("internalCreatureTeleport - Could not remove creature from current tile")
			return
		}
		fmt.Printf("internalCreatureTeleport - AddCreature - %v\n", _to.Position.String())
		if ret = _to.AddCreature(_creature, false); ret == RET_NOTPOSSIBLE {
			println("internalCreatureTeleport - Could not add creature to destination tile")
			_from.AddCreature(_creature, false) // Something went wrong, put creature back on old tile
			return
		}

		println("internalCreatureTeleport - Assign the new tile to the creature")
		_creature.SetTile(_to)

		// Tell creatures this creature has been teleported
		g.mutexCreatureList.RLock()
		defer g.mutexCreatureList.RUnlock()
		println("internalCreatureTeleport - Send the teleport to other clients")
		for _, value := range g.Creatures {
			if value != nil {
				value.OnCreatureMove(_creature, _from, _to, true)
			}
		}
	}
	
	println("internalCreatureTeleport - END");

	return
}

func (g *Game) internalCreatureSay(_creature ICreature, _speakType int, _message string) bool {
	list := make(CreatureList)
	if _speakType == pnet.SPEAK_YELL {
		position := _creature.GetPosition()  // Get position of speaker

		g.mutexPlayerList.RLock()
		defer g.mutexPlayerList.RUnlock()
		for _, player := range g.Players {
			if player != nil {
				if position.IsInRange3p(player.GetPosition(), pos.NewPositionFrom(27, 21, 0)) {
					list[player.GetUID()] = player
				}
			}
		}
	} else {
		list = _creature.GetVisibleCreatures()
	}
	
	// Send chat message to all visible players
	for _, creature := range list {
		if creature.GetType() == CTYPE_PLAYER {
			player := creature.(*Player)
			player.sendCreatureSay(_creature, _speakType, _message)
		}
	}	
	
	return true
}

func (g *Game) playerYell(_player *Player, _message string) bool {
	addExhaustion := 0
	isExhausted := false
	
	if !_player.HasCondition(CONDITION_EXHAUST_YELL, true) {
		addExhaustion = 10
		g.internalCreatureSay(_player, pnet.SPEAK_YELL, strings.ToUpper(_message))
	} else {
		isExhausted = true
		addExhaustion = 5
		_player.sendCancelMessage(RET_YOUAREEXHAUSTED)
	}
	
	if addExhaustion > 0 {
		condition := CreateCondition(CONDITIONID_DEFAULT, CONDITION_EXHAUST_YELL, addExhaustion, 0)
		_player.AddCondition(condition)
	}
	
	return !isExhausted
}

func (g *Game) playerWhisper(_player *Player, _text string) bool {
	list := _player.GetVisibleCreatures()
	position := _player.GetPosition()
	
	for _, creature := range list {
		if creature.GetType() == CTYPE_PLAYER {
			tmpPlayer := creature.(*Player)
			if position.IsInRange3p(tmpPlayer.GetPosition(), pos.NewPositionFrom(1, 1, 0)) {
				tmpPlayer.sendCreatureSay(_player, pnet.SPEAK_WHISPER, _text)
			} else {
				tmpPlayer.sendCreatureSay(_player, pnet.SPEAK_WHISPER, "pspsps")
			}
		}
	}
	
	return true
}

func (g *Game) playerSpeakTo(_player *Player, _type int, _receiver string, _text string) bool {
	toPlayer, found := g.GetPlayerByName(_receiver)
	if !found {
		_player.sendTextMessage(pnet.MSG_STATUS_SMALL, "A player with this name is not online.")
		return false
	}
	
	toPlayer.sendCreatureSay(_player, _type, _text)
	_player.sendTextMessage(pnet.MSG_STATUS_SMALL, fmt.Sprintf("Message sent to %v", toPlayer.GetName()))
	
	return true
}

func (g *Game) playerTalkToChannel(_player *Player, _type int, _text string, _channelId int) bool {
	return g.Chat.TalkToChannel(_player, _type, _text, _channelId)
}

func (g *Game) internalBroadcastMessage(_creature ICreature, _message string) bool {
	// TOOD: Check if _creature is CanBroadcast player flag

	g.mutexPlayerList.RLock()
	defer g.mutexPlayerList.RUnlock()
	for _, player := range g.Players {
		player.sendCreatureSay(_creature, pnet.SPEAK_BROADCAST, _message)
	}
	
	return true
}

func (g *Game) internalCreatureChangeVisible(_creature ICreature, _visible bool) {
	list := _creature.GetVisibleCreatures()
	for _, tmpCreature := range list {
		if tmpCreature.GetType() == CTYPE_PLAYER {
			tmpCreature.(*Player).sendCreatureChangeVisibility(_creature, _visible)
		}
	}
}

func (g *Game) checkForWildEncounter(_creature ICreature) {
	if _creature.GetType() == CTYPE_PLAYER {
		// Do some checkin'
	}
}
