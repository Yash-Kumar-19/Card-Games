package game

import "github.com/nakad/cardgames/internal/model"

// ActionType represents the type of action a player can take.
type ActionType string

const (
	ActionBlind ActionType = "blind"
	ActionSeen  ActionType = "seen"
	ActionCall  ActionType = "call"
	ActionRaise ActionType = "raise"
	ActionFold  ActionType = "fold"
	ActionShow  ActionType = "show"
)

// Action represents a player's action with an optional amount.
type Action struct {
	Type   ActionType
	Amount int64
}

// HandCategory represents the category/rank of a hand (game-specific ordering).
type HandCategory int

// HandRank represents the evaluated rank of a hand.
type HandRank struct {
	Category HandCategory
	// Tiebreakers are ordered from most to least significant.
	Tiebreakers []int
}

// GameState holds the mutable state of a game round.
type GameState struct {
	Deck          *model.Deck
	Players       []*model.Player
	ActivePlayers []string // IDs of players still in the round
	Hands         map[string][]model.Card
	Bets          map[string]int64
	Pot           int64
	CurrentBet    int64
	CurrentTurn   int
	DealerIndex   int
	BootAmount    int64
}

// Game is the central interface every card game must implement.
type Game interface {
	Name() string
	MinPlayers() int
	MaxPlayers() int
	CardsPerPlayer() int
	DealCards(state *GameState) error
	ValidActions(state *GameState, playerID string) []Action
	ApplyAction(state *GameState, playerID string, action Action) error
	EvaluateHand(cards []model.Card) HandRank
	CompareHands(a, b []model.Card) int
	IsRoundOver(state *GameState) bool
	DetermineWinner(state *GameState) []string
}
