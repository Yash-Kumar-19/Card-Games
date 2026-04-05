package model

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

type Suit int

const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

func (s Suit) String() string {
	return [...]string{"♠", "♥", "♦", "♣"}[s]
}

type Rank int

const (
	Two Rank = iota + 2
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

func (r Rank) String() string {
	switch {
	case r >= Two && r <= Ten:
		return fmt.Sprintf("%d", int(r))
	case r == Jack:
		return "J"
	case r == Queen:
		return "Q"
	case r == King:
		return "K"
	case r == Ace:
		return "A"
	default:
		return "?"
	}
}

type Card struct {
	Suit Suit
	Rank Rank
}

func (c Card) String() string {
	return fmt.Sprintf("%s%s", c.Rank, c.Suit)
}

type Deck struct {
	Cards []Card
}

func NewDeck() *Deck {
	cards := make([]Card, 0, 52)
	for suit := Spades; suit <= Clubs; suit++ {
		for rank := Two; rank <= Ace; rank++ {
			cards = append(cards, Card{Suit: suit, Rank: rank})
		}
	}
	return &Deck{Cards: cards}
}

// Shuffle performs a cryptographically secure Fisher-Yates shuffle.
func (d *Deck) Shuffle() error {
	for i := len(d.Cards) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("shuffle: %w", err)
		}
		j := n.Int64()
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i]
	}
	return nil
}

// Deal removes and returns the top n cards from the deck.
func (d *Deck) Deal(n int) ([]Card, error) {
	if n > len(d.Cards) {
		return nil, fmt.Errorf("deal: requested %d cards but only %d remain", n, len(d.Cards))
	}
	dealt := make([]Card, n)
	copy(dealt, d.Cards[:n])
	d.Cards = d.Cards[n:]
	return dealt, nil
}

// Remaining returns the number of cards left in the deck.
func (d *Deck) Remaining() int {
	return len(d.Cards)
}

// Player represents a player at a table.
type Player struct {
	ID        string
	Name      string
	Balance   int64
	Hand      []Card
	IsSeen    bool
	HasFolded bool
	IsActive  bool
}

func NewPlayer(id, name string, balance int64) *Player {
	return &Player{
		ID:       id,
		Name:     name,
		Balance:  balance,
		IsActive: true,
	}
}

func (p *Player) Reset() {
	p.Hand = nil
	p.IsSeen = false
	p.HasFolded = false
	p.IsActive = true
}
