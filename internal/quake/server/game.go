package server

import "fmt"

// GameType is the type of game.
type GameType int

const (
	FreeForAll     GameType = 0
	Tournament     GameType = 1
	SinglePlayer   GameType = 2
	TeamDeathmatch GameType = 3
	CaptureTheFlag GameType = 4
)

// String returns the string representation of the game type.
func (gameType GameType) String() string {
	switch gameType {
	case FreeForAll:
		return "FreeForAll"
	case Tournament:
		return "Tournament"
	case SinglePlayer:
		return "SinglePlayer"
	case TeamDeathmatch:
		return "TeamDeathmatch"
	case CaptureTheFlag:
		return "CaptureTheFlag"
	default:
		return "Unknown"
	}
}

// UnmarshalText unmarshals the game type from text.
func (gameType *GameType) UnmarshalText(data []byte) error {
	switch string(data) {
	case "FreeForAll", "FFA":
		*gameType = FreeForAll
	case "Tournament":
		*gameType = Tournament
	case "SinglePlayer":
		*gameType = SinglePlayer
	case "TeamDeathmatch":
		*gameType = TeamDeathmatch
	case "CaptureTheFlag", "CTF":
		*gameType = CaptureTheFlag
	default:
		return fmt.Errorf("unknown GameType: %s", data)
	}
	return nil
}
