// Copyright 2012-2013 The Ephenation Authors
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

package client_prot

//
// This defines the protocol used with the client.
// The first two bytes is the length of the complete message, with LSB first.
// The third byte always identifies the content of the message.
// The iota functionality could have been used, but a command number can never change as it would
// make client incompatible.
//
// For detailed description of the protocol, see the document in google docs.

const (
	CMD_LOGIN                      = 1  // The argument is a string with the login name
	CMD_SAVE                       = 2  // No argument
	CMD_QUIT                       = 3  // No argument
	CMD_MESSAGE                    = 4  // A text message to the client
	CMD_GET_COORDINATE             = 5  // Used to request the player coordinates (no arguments)
	CMD_REPORT_COORDINATE          = 6  // The answer sent from the server. Format is 3 x 8 bytes, with x, y, z, LSB first.
	CMD_READ_CHUNK                 = 7  // Read chunk at offset x,y,z from current chunk. Coded as 4 bytes, will be multiplied by chunk_size
	CMD_CHUNK_ANSWER               = 8  // The requested chunk sent back to the client
	CMD_LOGIN_ACK                  = 9  // Acknowledge login to the client
	CMD_START_FWD                  = 10 // Player start moving forward
	CMD_STOP_FWD                   = 11 // Player stop moving forward
	CMD_START_BWD                  = 12 // Player start moving backward
	CMD_STOP_BWD                   = 13 // Player stop moving backward'
	CMD_START_LFT                  = 14 // Player start moving left
	CMD_STOP_LFT                   = 15 // Player stop moving left
	CMD_START_RGT                  = 16 // Player start moving right
	CMD_STOP_RGT                   = 17 // Player stop moving right
	CMD_JUMP                       = 18 // Player jump
	CMD_SET_DIR                    = 19 // For the client to set the looking direction.
	CMD_OBJECT_LIST                = 20 // A list of players, mobs, and other moving objects positions
	CMD_HIT_BLOCK                  = 21 // The client registers a hit on an object
	CMD_BLOCK_UPDATE               = 22 // One or more blocks have been updated in a chunk
	CMD_DEBUG                      = 23 // A string sent to the server, interpreted as a debug command
	CMD_REQ_PASSWORD               = 24 // Request the password from the client, encrypt it with RC4 using argument.
	CMD_RESP_PASSWORD              = 25 // An encrypted password from the client to the server
	CMD_PROT_VERSION               = 26 // The version of the communication protocol
	CMD_VRFY_SUPERCHUNCK_CS        = 29 // Request server to verify one or more super chunk checksums. If wrong, an update will be sent.
	CMD_SUPERCHUNK_ANSWER          = 29 // (Same number) The full requested super chunk sent back to the client
	CMD_PLAYER_STATS               = 30 // Information about the player that doesn't change very often.
	CMD_ATTACK_MONSTER             = 31 // Initiate an attack on a monster
	CMD_PLAYER_ACTION              = 32 // A generic action, see UserAction* below
	CMD_RESP_PLAYER_HIT_BY_MONSTER = 33 // The player was hit by one or more monsters
	CMD_RESP_PLAYER_HIT_MONSTER    = 34 // The player hit one or more monsters
	CMD_RESP_AGGRO_FROM_MONSTER    = 35 // The player got aggro from one or more monsters
	CMD_VRFY_CHUNCK_CS             = 36 // Request server to verify checksum for one or more chunks. If wrong, the updated chunk will be sent.
	CMD_USE_ITEM                   = 37 // The player uses an item from the inventory.
	CMD_UPD_INV                    = 38 // Server updates the client about the amount of items.
	CMD_EQUIPMENT                  = 39 // Report equipment
	CMD_JELLY_BLOCKS               = 40 // Turn blocks transparent and permeable
	CMD_PING                       = 41 // Used to measure communication delay.
	CMD_DROP_ITEM                  = 42 // Drop an item from the inventory
	CMD_LOGINFAILED                = 43 // The login failed.
	CMD_REQ_PLAYER_INFO            = 44 // Request player information
	CMD_RESP_PLAYER_NAME           = 45 // A name of a player
	CMD_TELEPORT                   = 46 // Teleport player to a chunk coordinate.
	CMD_ERROR_REPORT               = 47 // Send an error report to the server, in the form of a string.
	CMD_Last                       = 48 // ONE HIGHER THAN LAST COMMAND! Add no commands after this one.

	ProtVersionMajor = 5
	ProtVersionMinor = 2
)

//
// These are the object types.
//
const (
	// Types used by OBJECT_LIST
	ObjTypePlayer  = 0
	ObjTypeMonster = 1

	// States used by OBJECT_LIST
	ObjStateRemove = 0
	ObjStateInGame = 1

	NEAR_OBJECTS    = 64  // All objects within this distance are considered to be near, and reported to clients
	BLOCK_COORD_RES = 100 // Fixed point resolution of coordinates

	// Action used as argument for the CMD_PLAYER_ACTION
	UserActionHeal       = 0
	UserActionCombAttack = 1
)

// These are flags used in the CMD_PLAYER_STATS command
const (
	UserFlagInFight = uint32(1 << 0)
	// UserFlagPlayerHit  = uint32(1 << 1) // The player hit a monster, transient flag
	// UserFlagMonsterHit = uint32(1 << 2) // A monster hit the player, transient flag
	UserFlagHealed = uint32(1 << 3) // The player was healed (not the gradual healing)
	UserFlagJump   = uint32(1 << 4) // The sound when the user hits the ground after jumping

	// Define a bit mask with all transient flags
	UserFlagTransientMask = (UserFlagHealed | UserFlagJump)
)
