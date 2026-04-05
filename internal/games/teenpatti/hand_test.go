package teenpatti

import (
	"testing"

	"github.com/nakad/cardgames/internal/game"
	"github.com/nakad/cardgames/internal/model"
)

func cards(defs ...struct {
	r model.Rank
	s model.Suit
}) []model.Card {
	out := make([]model.Card, len(defs))
	for i, d := range defs {
		out[i] = model.Card{Rank: d.r, Suit: d.s}
	}
	return out
}

func c(r model.Rank, s model.Suit) struct {
	r model.Rank
	s model.Suit
} {
	return struct {
		r model.Rank
		s model.Suit
	}{r, s}
}

func TestTrail(t *testing.T) {
	hand := cards(c(model.Ace, model.Spades), c(model.Ace, model.Hearts), c(model.Ace, model.Diamonds))
	hr := EvaluateHand(hand)
	if hr.Category != Trail {
		t.Fatalf("expected Trail, got %d (%s)", hr.Category, CategoryName(hr.Category))
	}
}

func TestPureSequence(t *testing.T) {
	hand := cards(c(model.King, model.Hearts), c(model.Queen, model.Hearts), c(model.Jack, model.Hearts))
	hr := EvaluateHand(hand)
	if hr.Category != PureSequence {
		t.Fatalf("expected PureSequence, got %s", CategoryName(hr.Category))
	}
}

func TestSequence(t *testing.T) {
	hand := cards(c(model.Five, model.Spades), c(model.Four, model.Hearts), c(model.Three, model.Diamonds))
	hr := EvaluateHand(hand)
	if hr.Category != Sequence {
		t.Fatalf("expected Sequence, got %s", CategoryName(hr.Category))
	}
}

func TestSequenceWrapAround(t *testing.T) {
	hand := cards(c(model.Ace, model.Spades), c(model.Two, model.Hearts), c(model.Three, model.Diamonds))
	hr := EvaluateHand(hand)
	if hr.Category != Sequence {
		t.Fatalf("expected Sequence (A-2-3), got %s", CategoryName(hr.Category))
	}

	// A-2-3 should beat 2-3-4 (Ace is highest)
	hand2 := cards(c(model.Four, model.Spades), c(model.Three, model.Hearts), c(model.Two, model.Diamonds))
	hr2 := EvaluateHand(hand2)
	result := CompareHands(hr, hr2)
	if result != 1 {
		t.Fatalf("expected A-2-3 > 2-3-4, got %d", result)
	}

	// A-K-Q should still beat A-2-3
	hand3 := cards(c(model.Ace, model.Spades), c(model.King, model.Hearts), c(model.Queen, model.Diamonds))
	hr3 := EvaluateHand(hand3)
	result2 := CompareHands(hr3, hr)
	if result2 != 1 {
		t.Fatalf("expected A-K-Q > A-2-3, got %d", result2)
	}
}

func TestColor(t *testing.T) {
	hand := cards(c(model.King, model.Diamonds), c(model.Nine, model.Diamonds), c(model.Five, model.Diamonds))
	hr := EvaluateHand(hand)
	if hr.Category != Color {
		t.Fatalf("expected Color, got %s", CategoryName(hr.Category))
	}
}

func TestPair(t *testing.T) {
	hand := cards(c(model.Jack, model.Spades), c(model.Jack, model.Hearts), c(model.Five, model.Diamonds))
	hr := EvaluateHand(hand)
	if hr.Category != Pair {
		t.Fatalf("expected Pair, got %s", CategoryName(hr.Category))
	}
}

func TestPairLowKicker(t *testing.T) {
	hand := cards(c(model.Five, model.Spades), c(model.Five, model.Hearts), c(model.Two, model.Diamonds))
	hr := EvaluateHand(hand)
	if hr.Category != Pair {
		t.Fatalf("expected Pair, got %s", CategoryName(hr.Category))
	}
}

func TestHighCard(t *testing.T) {
	hand := cards(c(model.King, model.Spades), c(model.Nine, model.Hearts), c(model.Five, model.Diamonds))
	hr := EvaluateHand(hand)
	if hr.Category != HighCard {
		t.Fatalf("expected HighCard, got %s", CategoryName(hr.Category))
	}
}

func TestHandRanking(t *testing.T) {
	trail := EvaluateHand(cards(c(model.Seven, model.Spades), c(model.Seven, model.Hearts), c(model.Seven, model.Diamonds)))
	pureSeq := EvaluateHand(cards(c(model.King, model.Hearts), c(model.Queen, model.Hearts), c(model.Jack, model.Hearts)))
	seq := EvaluateHand(cards(c(model.Five, model.Spades), c(model.Four, model.Hearts), c(model.Three, model.Diamonds)))
	color := EvaluateHand(cards(c(model.King, model.Diamonds), c(model.Nine, model.Diamonds), c(model.Five, model.Diamonds)))
	pair := EvaluateHand(cards(c(model.Jack, model.Spades), c(model.Jack, model.Hearts), c(model.Five, model.Diamonds)))
	high := EvaluateHand(cards(c(model.King, model.Spades), c(model.Nine, model.Hearts), c(model.Five, model.Diamonds)))

	rankings := []game.HandRank{trail, pureSeq, seq, color, pair, high}
	for i := 0; i < len(rankings)-1; i++ {
		result := CompareHands(rankings[i], rankings[i+1])
		if result != 1 {
			t.Fatalf("expected rank %d > rank %d, got %d", i, i+1, result)
		}
	}
}

func TestCompareHandsTiebreaker(t *testing.T) {
	a := EvaluateHand(cards(c(model.King, model.Spades), c(model.King, model.Hearts), c(model.Ace, model.Diamonds)))
	b := EvaluateHand(cards(c(model.King, model.Diamonds), c(model.King, model.Clubs), c(model.Queen, model.Spades)))

	if CompareHands(a, b) != 1 {
		t.Fatal("expected pair of Kings with Ace kicker > pair of Kings with Queen kicker")
	}
}

// TestAceIsHighestInEveryCategory verifies Ace is the highest card across all hand types.
func TestAceIsHighestInEveryCategory(t *testing.T) {
	// Trail: A-A-A > K-K-K
	aceTrail := EvaluateHand(cards(c(model.Ace, model.Spades), c(model.Ace, model.Hearts), c(model.Ace, model.Diamonds)))
	kingTrail := EvaluateHand(cards(c(model.King, model.Spades), c(model.King, model.Hearts), c(model.King, model.Diamonds)))
	if CompareHands(aceTrail, kingTrail) != 1 {
		t.Fatal("Trail: A-A-A should beat K-K-K")
	}

	// Pure Sequence: A-K-Q suited > K-Q-J suited
	acePureSeq := EvaluateHand(cards(c(model.Ace, model.Hearts), c(model.King, model.Hearts), c(model.Queen, model.Hearts)))
	kingPureSeq := EvaluateHand(cards(c(model.King, model.Spades), c(model.Queen, model.Spades), c(model.Jack, model.Spades)))
	if CompareHands(acePureSeq, kingPureSeq) != 1 {
		t.Fatal("Pure Sequence: A-K-Q suited should beat K-Q-J suited")
	}

	// Pure Sequence: A-2-3 suited > K-Q-J suited
	a23PureSeq := EvaluateHand(cards(c(model.Ace, model.Hearts), c(model.Two, model.Hearts), c(model.Three, model.Hearts)))
	if CompareHands(a23PureSeq, kingPureSeq) != 1 {
		t.Fatal("Pure Sequence: A-2-3 suited should beat K-Q-J suited")
	}

	// Sequence: A-K-Q > K-Q-J
	aceSeq := EvaluateHand(cards(c(model.Ace, model.Spades), c(model.King, model.Hearts), c(model.Queen, model.Diamonds)))
	kingSeq := EvaluateHand(cards(c(model.King, model.Spades), c(model.Queen, model.Hearts), c(model.Jack, model.Diamonds)))
	if CompareHands(aceSeq, kingSeq) != 1 {
		t.Fatal("Sequence: A-K-Q should beat K-Q-J")
	}

	// Sequence: A-2-3 > K-Q-J
	a23Seq := EvaluateHand(cards(c(model.Ace, model.Spades), c(model.Two, model.Hearts), c(model.Three, model.Diamonds)))
	if CompareHands(a23Seq, kingSeq) != 1 {
		t.Fatal("Sequence: A-2-3 should beat K-Q-J")
	}

	// Sequence ranking: A-K-Q > A-2-3 > K-Q-J > ... > 4-3-2
	if CompareHands(aceSeq, a23Seq) != 1 {
		t.Fatal("Sequence: A-K-Q should beat A-2-3")
	}
	lowSeq := EvaluateHand(cards(c(model.Four, model.Spades), c(model.Three, model.Hearts), c(model.Two, model.Diamonds)))
	if CompareHands(a23Seq, lowSeq) != 1 {
		t.Fatal("Sequence: A-2-3 should beat 4-3-2")
	}

	// Color: Ace-high flush > King-high flush
	aceColor := EvaluateHand(cards(c(model.Ace, model.Diamonds), c(model.Nine, model.Diamonds), c(model.Five, model.Diamonds)))
	kingColor := EvaluateHand(cards(c(model.King, model.Clubs), c(model.Nine, model.Clubs), c(model.Five, model.Clubs)))
	if CompareHands(aceColor, kingColor) != 1 {
		t.Fatal("Color: Ace-high should beat King-high")
	}

	// Pair: A-A-x > K-K-x
	acePair := EvaluateHand(cards(c(model.Ace, model.Spades), c(model.Ace, model.Hearts), c(model.Two, model.Diamonds)))
	kingPair := EvaluateHand(cards(c(model.King, model.Spades), c(model.King, model.Hearts), c(model.Queen, model.Diamonds)))
	if CompareHands(acePair, kingPair) != 1 {
		t.Fatal("Pair: A-A should beat K-K")
	}

	// High Card: Ace-high > King-high
	aceHigh := EvaluateHand(cards(c(model.Ace, model.Spades), c(model.Seven, model.Hearts), c(model.Three, model.Diamonds)))
	kingHigh := EvaluateHand(cards(c(model.King, model.Spades), c(model.Seven, model.Hearts), c(model.Three, model.Diamonds)))
	if CompareHands(aceHigh, kingHigh) != 1 {
		t.Fatal("High Card: Ace-high should beat King-high")
	}
}
