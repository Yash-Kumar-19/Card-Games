package engine

import (
	"testing"

	"github.com/nakad/cardgames/internal/model"
)

func TestNewDeck(t *testing.T) {
	d := model.NewDeck()
	if len(d.Cards) != 52 {
		t.Fatalf("expected 52 cards, got %d", len(d.Cards))
	}

	// Check uniqueness
	seen := make(map[model.Card]bool)
	for _, c := range d.Cards {
		if seen[c] {
			t.Fatalf("duplicate card: %s", c)
		}
		seen[c] = true
	}
}

func TestShuffle(t *testing.T) {
	d := model.NewDeck()
	original := make([]model.Card, 52)
	copy(original, d.Cards)

	if err := d.Shuffle(); err != nil {
		t.Fatalf("shuffle error: %v", err)
	}

	if len(d.Cards) != 52 {
		t.Fatalf("expected 52 cards after shuffle, got %d", len(d.Cards))
	}

	// Check that shuffle changed the order (extremely unlikely to be same)
	same := true
	for i := range d.Cards {
		if d.Cards[i] != original[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("shuffle did not change card order (statistically near-impossible)")
	}

	// Still has all unique cards
	seen := make(map[model.Card]bool)
	for _, c := range d.Cards {
		if seen[c] {
			t.Fatalf("duplicate card after shuffle: %s", c)
		}
		seen[c] = true
	}
}

func TestDeal(t *testing.T) {
	d := model.NewDeck()
	_ = d.Shuffle()

	cards, err := d.Deal(3)
	if err != nil {
		t.Fatalf("deal error: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(cards))
	}
	if d.Remaining() != 49 {
		t.Fatalf("expected 49 remaining, got %d", d.Remaining())
	}

	// Deal too many
	_, err = d.Deal(50)
	if err == nil {
		t.Fatal("expected error when dealing more cards than available")
	}
}

func TestCardString(t *testing.T) {
	c := model.Card{Suit: model.Hearts, Rank: model.Ace}
	s := c.String()
	if s != "A♥" {
		t.Fatalf("expected A♥, got %s", s)
	}

	c2 := model.Card{Suit: model.Spades, Rank: model.Ten}
	if c2.String() != "10♠" {
		t.Fatalf("expected 10♠, got %s", c2.String())
	}
}
