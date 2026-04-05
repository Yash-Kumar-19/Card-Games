package teenpatti

import (
	"sort"

	"github.com/nakad/cardgames/internal/game"
	"github.com/nakad/cardgames/internal/model"
)

// Hand categories for Teen Patti, ordered from lowest to highest.
const (
	HighCard game.HandCategory = iota
	Pair
	Color
	Sequence
	PureSequence
	Trail
)

func CategoryName(c game.HandCategory) string {
	switch c {
	case Trail:
		return "Trail"
	case PureSequence:
		return "Pure Sequence"
	case Sequence:
		return "Sequence"
	case Color:
		return "Color"
	case Pair:
		return "Pair"
	case HighCard:
		return "High Card"
	default:
		return "Unknown"
	}
}

// EvaluateHand evaluates a 3-card Teen Patti hand and returns its rank.
func EvaluateHand(cards []model.Card) game.HandRank {
	if len(cards) != 3 {
		return game.HandRank{Category: HighCard}
	}

	// Sort by rank descending for consistent comparison.
	sorted := make([]model.Card, 3)
	copy(sorted, cards)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Rank > sorted[j].Rank
	})

	ranks := [3]model.Rank{sorted[0].Rank, sorted[1].Rank, sorted[2].Rank}
	sameSuit := sorted[0].Suit == sorted[1].Suit && sorted[1].Suit == sorted[2].Suit
	isSeq := isSequence(ranks)

	// Trail (three of a kind)
	if ranks[0] == ranks[1] && ranks[1] == ranks[2] {
		return game.HandRank{
			Category:    Trail,
			Tiebreakers: []int{int(ranks[0])},
		}
	}

	// Pure Sequence (straight flush)
	if sameSuit && isSeq {
		return game.HandRank{
			Category:    PureSequence,
			Tiebreakers: seqTiebreakers(ranks),
		}
	}

	// Sequence (straight)
	if isSeq {
		return game.HandRank{
			Category:    Sequence,
			Tiebreakers: seqTiebreakers(ranks),
		}
	}

	// Color (flush)
	if sameSuit {
		return game.HandRank{
			Category:    Color,
			Tiebreakers: []int{int(ranks[0]), int(ranks[1]), int(ranks[2])},
		}
	}

	// Pair
	if ranks[0] == ranks[1] {
		return game.HandRank{
			Category:    Pair,
			Tiebreakers: []int{int(ranks[0]), int(ranks[2])}, // pair rank, then kicker
		}
	}
	if ranks[1] == ranks[2] {
		return game.HandRank{
			Category:    Pair,
			Tiebreakers: []int{int(ranks[1]), int(ranks[0])},
		}
	}
	if ranks[0] == ranks[2] {
		return game.HandRank{
			Category:    Pair,
			Tiebreakers: []int{int(ranks[0]), int(ranks[1])},
		}
	}

	// High card
	return game.HandRank{
		Category:    HighCard,
		Tiebreakers: []int{int(ranks[0]), int(ranks[1]), int(ranks[2])},
	}
}

// isSequence checks if three ranks (sorted descending) form a sequence.
func isSequence(ranks [3]model.Rank) bool {
	if ranks[0]-ranks[1] == 1 && ranks[1]-ranks[2] == 1 {
		return true
	}
	// Special wrap-around: A-2-3
	if ranks[0] == model.Ace && ranks[1] == model.Three && ranks[2] == model.Two {
		return true
	}
	return false
}

// seqTiebreakers returns tiebreaker values for a sequence.
func seqTiebreakers(ranks [3]model.Rank) []int {
	if ranks[0] == model.Ace && ranks[1] == model.Three && ranks[2] == model.Two {
		return []int{int(model.Ace), int(model.Three)}
	}
	return []int{int(ranks[0]), int(ranks[1])}
}

// CompareHands compares two evaluated hand ranks.
func CompareHands(a, b game.HandRank) int {
	if a.Category != b.Category {
		if a.Category > b.Category {
			return 1
		}
		return -1
	}
	// Same category — compare tiebreakers
	for i := 0; i < len(a.Tiebreakers) && i < len(b.Tiebreakers); i++ {
		if a.Tiebreakers[i] > b.Tiebreakers[i] {
			return 1
		}
		if a.Tiebreakers[i] < b.Tiebreakers[i] {
			return -1
		}
	}
	return 0
}
