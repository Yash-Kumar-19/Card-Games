package ws

import "github.com/nakad/cardgames/internal/game"

// EventType represents a WebSocket event.
type EventType string

const (
	EventJoinTable    EventType = "JOIN_TABLE"
	EventLeaveTable   EventType = "LEAVE_TABLE"
	EventStartGame    EventType = "START_GAME"
	EventDealCards    EventType = "DEAL_CARDS"
	EventPlayerAction EventType = "PLAYER_ACTION"
	EventTurnChange   EventType = "TURN_CHANGE"
	EventGameResult   EventType = "GAME_RESULT"
	EventError        EventType = "ERROR"
	EventTableState   EventType = "TABLE_STATE"
	EventPlayerJoined EventType = "PLAYER_JOINED"
	EventPlayerLeft   EventType = "PLAYER_LEFT"
)

// ClientMessage is a message sent from client to server.
type ClientMessage struct {
	Type    EventType     `json:"type"`
	TableID string        `json:"table_id,omitempty"`
	Action  *ClientAction `json:"action,omitempty"`
	Create  *CreateTable  `json:"create,omitempty"`
}

// ClientAction is a player action sent from the client.
type ClientAction struct {
	Type   game.ActionType `json:"type"`
	Amount int64           `json:"amount,omitempty"`
}

// CreateTable is a request to create a new table.
type CreateTable struct {
	GameType   string `json:"game_type"`
	BootAmount int64  `json:"boot_amount"`
}

// ServerMessage is a message sent from server to client.
type ServerMessage struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// DealPayload is sent to each player with their own cards.
type DealPayload struct {
	Cards      []CardDTO        `json:"cards"`
	Pot        int64            `json:"pot"`
	CurrentBet int64            `json:"current_bet"`
	Players    []PlayerStateDTO `json:"players"`
}

// CardDTO is the JSON-safe representation of a card.
type CardDTO struct {
	Rank string `json:"rank"`
	Suit string `json:"suit"`
}

// PlayerStateDTO is a public view of a player at the table.
type PlayerStateDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Balance   int64  `json:"balance"`
	IsSeen    bool   `json:"is_seen"`
	HasFolded bool   `json:"has_folded"`
	IsActive  bool   `json:"is_active"`
	CardCount int    `json:"card_count"` // how many cards they hold (no reveal)
}

// TurnPayload notifies whose turn it is.
type TurnPayload struct {
	PlayerID   string `json:"player_id"`
	CurrentBet int64  `json:"current_bet"`
	Pot        int64  `json:"pot"`
	TimeoutSec int    `json:"timeout_sec"`
}

// ResultPayload is the game result.
type ResultPayload struct {
	Winners []WinnerDTO          `json:"winners"`
	Pot     int64                `json:"pot"`
	Hands   map[string][]CardDTO `json:"hands"` // reveal all hands
}

// WinnerDTO describes a winner.
type WinnerDTO struct {
	PlayerID string    `json:"player_id"`
	Name     string    `json:"name"`
	Hand     []CardDTO `json:"hand"`
	HandName string    `json:"hand_name"`
}

// TableStatePayload is the full table state for reconnection / initial join.
type TableStatePayload struct {
	TableID     string           `json:"table_id"`
	GameType    string           `json:"game_type"`
	State       string           `json:"state"`
	Players     []PlayerStateDTO `json:"players"`
	Pot         int64            `json:"pot"`
	CurrentBet  int64            `json:"current_bet"`
	CurrentTurn string           `json:"current_turn,omitempty"`
	BootAmount  int64            `json:"boot_amount"`
}
