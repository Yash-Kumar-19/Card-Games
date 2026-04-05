package engine

import (
	"testing"

	"github.com/nakad/cardgames/internal/game"
	"github.com/nakad/cardgames/internal/games/teenpatti"
	"github.com/nakad/cardgames/internal/model"
)

func TestTableLifecycle(t *testing.T) {
	tp := teenpatti.New()
	table := NewTable("table-1", tp, 10)

	p1 := model.NewPlayer("p1", "Alice", 1000)
	p2 := model.NewPlayer("p2", "Bob", 1000)
	p3 := model.NewPlayer("p3", "Charlie", 1000)

	if err := table.AddPlayer(p1); err != nil {
		t.Fatal(err)
	}
	if err := table.AddPlayer(p2); err != nil {
		t.Fatal(err)
	}
	if err := table.AddPlayer(p3); err != nil {
		t.Fatal(err)
	}

	// Cannot add duplicate
	if err := table.AddPlayer(p1); err == nil {
		t.Fatal("expected error adding duplicate player")
	}

	// Start round
	if err := table.StartRound(); err != nil {
		t.Fatal(err)
	}

	if table.State != StateBetting {
		t.Fatalf("expected BETTING state, got %s", table.State)
	}

	// Each player should have 3 cards
	for _, p := range table.Players {
		if len(p.Hand) != 3 {
			t.Fatalf("player %s has %d cards, expected 3", p.ID, len(p.Hand))
		}
	}

	// Boot should have been collected
	expectedPot := int64(30) // 3 players * 10
	if table.GameState.Pot != expectedPot {
		t.Fatalf("expected pot %d, got %d", expectedPot, table.GameState.Pot)
	}

	// Each player should have 990 balance
	for _, p := range table.Players {
		if p.Balance != 990 {
			t.Fatalf("player %s balance is %d, expected 990", p.ID, p.Balance)
		}
	}
}

func TestTableFoldToWin(t *testing.T) {
	tp := teenpatti.New()
	table := NewTable("table-2", tp, 10)

	p1 := model.NewPlayer("p1", "Alice", 1000)
	p2 := model.NewPlayer("p2", "Bob", 1000)
	p3 := model.NewPlayer("p3", "Charlie", 1000)
	_ = table.AddPlayer(p1)
	_ = table.AddPlayer(p2)
	_ = table.AddPlayer(p3)
	_ = table.StartRound()

	// Fold the first two players in turn order, last one wins
	gs := table.GameState

	// First player folds
	firstPlayer := gs.ActivePlayers[gs.CurrentTurn]
	if err := table.PlayerAction(firstPlayer, game.Action{Type: game.ActionFold}); err != nil {
		t.Fatal(err)
	}

	// Second player folds (current turn updated after first fold)
	secondPlayer := gs.ActivePlayers[gs.CurrentTurn]
	if err := table.PlayerAction(secondPlayer, game.Action{Type: game.ActionFold}); err != nil {
		t.Fatal(err)
	}

	// The remaining player should be the winner
	if table.State != StateShowdown {
		t.Fatalf("expected SHOWDOWN, got %s", table.State)
	}

	winners, err := table.ResolveShowdown()
	if err != nil {
		t.Fatal(err)
	}
	if len(winners) != 1 {
		t.Fatalf("expected 1 winner, got %v", winners)
	}
	// Winner should not be either of the folded players
	if winners[0] == firstPlayer || winners[0] == secondPlayer {
		t.Fatalf("winner %s should not be a folded player", winners[0])
	}
}

func TestTableInsufficientPlayers(t *testing.T) {
	tp := teenpatti.New()
	table := NewTable("table-3", tp, 10)

	p1 := model.NewPlayer("p1", "Alice", 1000)
	_ = table.AddPlayer(p1)

	if err := table.StartRound(); err == nil {
		t.Fatal("expected error with insufficient players")
	}
}
