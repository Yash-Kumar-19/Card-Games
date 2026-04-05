package teenpatti

import (
	"fmt"

	"github.com/nakad/cardgames/internal/game"
	"github.com/nakad/cardgames/internal/model"
)

// TeenPatti implements the game.Game interface.
type TeenPatti struct{}

func New() *TeenPatti {
	return &TeenPatti{}
}

func (tp *TeenPatti) Name() string        { return "teen_patti" }
func (tp *TeenPatti) MinPlayers() int      { return 3 }
func (tp *TeenPatti) MaxPlayers() int      { return 6 }
func (tp *TeenPatti) CardsPerPlayer() int  { return 3 }

func (tp *TeenPatti) DealCards(state *game.GameState) error {
	for _, pid := range state.ActivePlayers {
		cards, err := state.Deck.Deal(3)
		if err != nil {
			return fmt.Errorf("deal to %s: %w", pid, err)
		}
		state.Hands[pid] = cards
		// Also set on the player object
		for _, p := range state.Players {
			if p.ID == pid {
				p.Hand = cards
				break
			}
		}
	}
	return nil
}

func (tp *TeenPatti) ValidActions(state *game.GameState, playerID string) []game.Action {
	player := findPlayer(state, playerID)
	if player == nil || player.HasFolded || !player.IsActive {
		return nil
	}

	var actions []game.Action

	if !player.IsSeen {
		// Blind player can: blind bet, see cards, or fold
		actions = append(actions,
			game.Action{Type: game.ActionBlind, Amount: state.CurrentBet},
			game.Action{Type: game.ActionSeen},
			game.Action{Type: game.ActionFold},
		)
	} else {
		// Seen player can: call, raise, fold, or show
		seenBet := state.CurrentBet * 2
		actions = append(actions,
			game.Action{Type: game.ActionCall, Amount: seenBet},
			game.Action{Type: game.ActionRaise, Amount: seenBet * 2},
			game.Action{Type: game.ActionFold},
		)
		// Show is only available when 2 players remain
		if len(state.ActivePlayers) == 2 {
			actions = append(actions, game.Action{Type: game.ActionShow, Amount: seenBet})
		}
	}

	return actions
}

func (tp *TeenPatti) ApplyAction(state *game.GameState, playerID string, action game.Action) error {
	player := findPlayer(state, playerID)
	if player == nil {
		return fmt.Errorf("player %s not found", playerID)
	}

	// Verify it's this player's turn
	if state.ActivePlayers[state.CurrentTurn] != playerID {
		return fmt.Errorf("not player %s's turn", playerID)
	}

	switch action.Type {
	case game.ActionBlind:
		if player.IsSeen {
			return fmt.Errorf("player %s has already seen cards, cannot play blind", playerID)
		}
		amt := state.CurrentBet
		if player.Balance < amt {
			return fmt.Errorf("insufficient balance: need %d, have %d", amt, player.Balance)
		}
		player.Balance -= amt
		state.Bets[playerID] += amt
		state.Pot += amt

	case game.ActionSeen:
		player.IsSeen = true
		// Looking at cards is free, no bet required
		return nil

	case game.ActionCall:
		if !player.IsSeen {
			return fmt.Errorf("blind player cannot call, use blind action")
		}
		amt := state.CurrentBet * 2 // seen players pay double
		if player.Balance < amt {
			return fmt.Errorf("insufficient balance: need %d, have %d", amt, player.Balance)
		}
		player.Balance -= amt
		state.Bets[playerID] += amt
		state.Pot += amt

	case game.ActionRaise:
		var amt int64
		if player.IsSeen {
			amt = state.CurrentBet * 2 * 2 // seen raise = 2x seen bet
		} else {
			amt = state.CurrentBet * 2 // blind raise = 2x blind bet
		}
		if action.Amount > 0 && action.Amount >= amt {
			amt = action.Amount
		}
		if player.Balance < amt {
			return fmt.Errorf("insufficient balance: need %d, have %d", amt, player.Balance)
		}
		player.Balance -= amt
		state.Bets[playerID] += amt
		state.Pot += amt
		// Update current bet: blind bet = half for blind players
		if player.IsSeen {
			state.CurrentBet = amt / 2
		} else {
			state.CurrentBet = amt
		}

	case game.ActionFold:
		player.HasFolded = true
		player.IsActive = false
		removeFromActive(state, playerID)

	case game.ActionShow:
		if len(state.ActivePlayers) != 2 {
			return fmt.Errorf("show is only allowed when 2 players remain")
		}
		amt := state.CurrentBet * 2 // pay show cost
		if player.IsSeen {
			if player.Balance < amt {
				return fmt.Errorf("insufficient balance: need %d, have %d", amt, player.Balance)
			}
			player.Balance -= amt
			state.Bets[playerID] += amt
			state.Pot += amt
		}
		// Show triggers showdown — don't advance turn
		return nil

	default:
		return fmt.Errorf("unknown action: %s", action.Type)
	}

	// Advance to next player (skip if fold already removed them)
	if action.Type != game.ActionFold || len(state.ActivePlayers) > 0 {
		advanceTurn(state)
	}

	return nil
}

func (tp *TeenPatti) EvaluateHand(cards []model.Card) game.HandRank {
	return EvaluateHand(cards)
}

func (tp *TeenPatti) CompareHands(a, b []model.Card) int {
	return CompareHands(EvaluateHand(a), EvaluateHand(b))
}

func (tp *TeenPatti) IsRoundOver(state *game.GameState) bool {
	return len(state.ActivePlayers) <= 1
}

func (tp *TeenPatti) DetermineWinner(state *game.GameState) []string {
	if len(state.ActivePlayers) == 1 {
		return state.ActivePlayers
	}
	if len(state.ActivePlayers) == 0 {
		return nil
	}

	// Compare all remaining hands
	bestID := state.ActivePlayers[0]
	bestHand := EvaluateHand(state.Hands[bestID])

	for _, pid := range state.ActivePlayers[1:] {
		h := EvaluateHand(state.Hands[pid])
		if CompareHands(h, bestHand) > 0 {
			bestID = pid
			bestHand = h
		}
	}

	// Collect all players tied with the best
	winners := []string{}
	for _, pid := range state.ActivePlayers {
		h := EvaluateHand(state.Hands[pid])
		if CompareHands(h, bestHand) == 0 {
			winners = append(winners, pid)
		}
	}
	return winners
}

// --- helpers ---

func findPlayer(state *game.GameState, id string) *model.Player {
	for _, p := range state.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func removeFromActive(state *game.GameState, id string) {
	for i, pid := range state.ActivePlayers {
		if pid == id {
			state.ActivePlayers = append(state.ActivePlayers[:i], state.ActivePlayers[i+1:]...)
			// Adjust current turn if needed
			if state.CurrentTurn >= len(state.ActivePlayers) && len(state.ActivePlayers) > 0 {
				state.CurrentTurn = 0
			}
			return
		}
	}
}

func advanceTurn(state *game.GameState) {
	if len(state.ActivePlayers) == 0 {
		return
	}
	state.CurrentTurn = (state.CurrentTurn + 1) % len(state.ActivePlayers)
}
