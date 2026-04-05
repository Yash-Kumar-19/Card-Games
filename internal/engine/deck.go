// Package engine re-exports model types for convenience and contains
// table lifecycle logic.
package engine

import "github.com/nakad/cardgames/internal/model"

// Re-export model types so existing imports keep working within engine tests.
type Card = model.Card
type Deck = model.Deck
type Suit = model.Suit
type Rank = model.Rank

const (
	Spades   = model.Spades
	Hearts   = model.Hearts
	Diamonds = model.Diamonds
	Clubs    = model.Clubs
)

const (
	Two   = model.Two
	Three = model.Three
	Four  = model.Four
	Five  = model.Five
	Six   = model.Six
	Seven = model.Seven
	Eight = model.Eight
	Nine  = model.Nine
	Ten   = model.Ten
	Jack  = model.Jack
	Queen = model.Queen
	King  = model.King
	Ace   = model.Ace
)

var NewDeck = model.NewDeck
