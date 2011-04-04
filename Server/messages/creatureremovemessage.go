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
	"os"
	pnet "network"
)

type CreatureRemoveMessage struct {
	creature ICreature
}

func NewCreatureRemoveMessage(_creature ICreature) *CreatureRemoveMessage {
	return &CreatureRemoveMessage { creature: _creature }
}

// GetHeader returns the header value of this message
func (m *CreatureRemoveMessage) GetHeader() uint8 {
	return pnet.HEADER_REMOVECREATURE
}

// WritePacket write the needed object data to a Packet and returns it
func (m *CreatureRemoveMessage) WritePacket() (*pnet.Packet, os.Error) {
	packet := pnet.NewPacketExt(m.GetHeader())
	packet.AddUint64(m.creature.GetUID())
	
	return packet, nil
}
