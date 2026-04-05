package engine

import (
	"fmt"

	"github.com/nakad/cardgames/internal/game"
)

// TableState represents the current phase of a table.
type TableState int

const (
	StateWaiting  TableState = iota // Waiting for players
	StateStarting                   // About to start dealing
	StateDealing                    // Cards being dealt
	StateBetting                    // Active betting round
	StateShowdown                   // Comparing hands
	StateFinished                   // Round complete
)

func (s TableState) String() string {
	return [...]string{"WAITING", "STARTING", "DEALING", "BETTING", "SHOWDOWN", "FINISHED"}[s]
}

// Table represents a game table that manages the lifecycle of a game round.
type Table struct {
	ID          string
	Game        game.Game
	Players     []*Player
	State       TableState
	DealerIndex int
	GameState   *game.GameState
	BootAmount  int64
}

// NewTable creates a new table for the given game type.
func NewTable(id string, g game.Game, bootAmount int64) *Table {
	return &Table{
		ID:         id,
		Game:       g,
		State:      StateWaiting,
		BootAmount: bootAmount,
	}
}

// AddPlayer adds a player to the table.
func (t *Table) AddPlayer(p *Player) error {
	if t.State != StateWaiting {
		return fmt.Errorf("cannot join: table is in %s state", t.State)
	}
	if len(t.Players) >= t.Game.MaxPlayers() {
		return fmt.Errorf("table is full (%d/%d)", len(t.Players), t.Game.MaxPlayers())
	}
	for _, existing := range t.Players {
		if existing.ID == p.ID {
			return fmt.Errorf("player %s already at table", p.ID)
		}
	}
	t.Players = append(t.Players, p)
	return nil
}

// RemovePlayer removes a player from the table.
func (t *Table) RemovePlayer(playerID string) error {
	for i, p := range t.Players {
		if p.ID == playerID {
			t.Players = append(t.Players[:i], t.Players[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("player %s not found at table", playerID)
}

// StartRound transitions the table through dealing and into betting.
func (t *Table) StartRound() error {
	if t.State != StateWaiting && t.State != StateFinished {
		return fmt.Errorf("cannot start: table is in %s state", t.State)
	}
	if len(t.Players) < t.Game.MinPlayers() {
		return fmt.Errorf("need at least %d players, have %d", t.Game.MinPlayers(), len(t.Players))
	}

	// Reset players
	for _, p := range t.Players {
		p.Reset()
	}

	// Build active player list
	activeIDs := make([]string, len(t.Players))
	for i, p := range t.Players {
		activeIDs[i] = p.ID
	}

	// Create deck and shuffle
	deck := NewDeck()
	if err := deck.Shuffle(); err != nil {
		return fmt.Errorf("shuffle: %w", err)
	}

	// Initialize game state
	t.GameState = &game.GameState{
		Deck:          deck,
		Players:       t.Players,
		ActivePlayers: activeIDs,
		Hands:         make(map[string][]Card),
		Bets:          make(map[string]int64),
		Pot:           0,
		CurrentBet:    t.BootAmount,
		CurrentTurn:   0,
		DealerIndex:   t.DealerIndex,
		BootAmount:    t.BootAmount,
	}

	// Collect boot from all players
	t.State = StateStarting
	for _, p := range t.Players {
		if p.Balance < t.BootAmount {
			return fmt.Errorf("player %s has insufficient balance for boot (%d < %d)", p.ID, p.Balance, t.BootAmount)
		}
		p.Balance -= t.BootAmount
		t.GameState.Bets[p.ID] = t.BootAmount
		t.GameState.Pot += t.BootAmount
	}

	// Deal cards
	t.State = StateDealing
	if err := t.Game.DealCards(t.GameState); err != nil {
		return fmt.Errorf("deal: %w", err)
	}

	// Set first player (left of dealer)
	t.GameState.CurrentTurn = (t.DealerIndex + 1) % len(t.GameState.ActivePlayers)

	// Enter betting
	t.State = StateBetting
	return nil
}

// PlayerAction processes an action from a player.
func (t *Table) PlayerAction(playerID string, action game.Action) error {
	if t.State != StateBetting {
		return fmt.Errorf("cannot act: table is in %s state", t.State)
	}

	isShow := action.Type == game.ActionShow

	if err := t.Game.ApplyAction(t.GameState, playerID, action); err != nil {
		return err
	}

	// Check if round is over
	if t.Game.IsRoundOver(t.GameState) || isShow {
		t.State = StateShowdown
	}

	return nil
}

// ResolveShowdown determines the winner and distributes the pot.
func (t *Table) ResolveShowdown() ([]string, error) {
	if t.State != StateShowdown {
		return nil, fmt.Errorf("cannot resolve: table is in %s state", t.State)
	}

	winners := t.Game.DetermineWinner(t.GameState)
	if len(winners) == 0 {
		return nil, fmt.Errorf("no winner determined")
	}

	// Split pot among winners
	share := t.GameState.Pot / int64(len(winners))
	for _, wid := range winners {
		for _, p := range t.Players {
			if p.ID == wid {
				p.Balance += share
				break
			}
		}
	}

	t.State = StateFinished
	// Rotate dealer for next round
	t.DealerIndex = (t.DealerIndex + 1) % len(t.Players)

	return winners, nil
}
