package wallet

import (
	"context"
	"fmt"

	"github.com/nakad/cardgames/internal/store"
)

// Service handles all wallet operations: boot collection, bets, and winnings.
type Service struct {
	users  store.UserStore
	wallet store.WalletStore
}

// NewService creates a new wallet service.
func NewService(users store.UserStore, wallet store.WalletStore) *Service {
	return &Service{users: users, wallet: wallet}
}

// GetBalance returns the current balance for a user.
func (s *Service) GetBalance(ctx context.Context, userID string) (int64, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return u.Balance, nil
}

// CollectBoot deducts the boot amount from a player's balance.
func (s *Service) CollectBoot(ctx context.Context, userID string, amount int64, tableID string) error {
	_, err := s.wallet.DebitWithCheck(ctx, userID, amount, "boot", tableID)
	if err != nil {
		return fmt.Errorf("collect boot for %s: %w", userID, err)
	}
	return nil
}

// PlaceBet deducts a bet amount from a player's balance.
func (s *Service) PlaceBet(ctx context.Context, userID string, amount int64, tableID string) error {
	_, err := s.wallet.DebitWithCheck(ctx, userID, amount, "bet", tableID)
	if err != nil {
		return fmt.Errorf("place bet for %s: %w", userID, err)
	}
	return nil
}

// CreditWinnings adds pot winnings to a player's balance.
func (s *Service) CreditWinnings(ctx context.Context, userID string, amount int64, tableID string) error {
	_, err := s.wallet.Credit(ctx, userID, amount, "win", tableID)
	if err != nil {
		return fmt.Errorf("credit winnings for %s: %w", userID, err)
	}
	return nil
}

// RefundBoot returns boot amount when a game is cancelled.
func (s *Service) RefundBoot(ctx context.Context, userID string, amount int64, tableID string) error {
	_, err := s.wallet.Credit(ctx, userID, amount, "refund", tableID)
	if err != nil {
		return fmt.Errorf("refund boot for %s: %w", userID, err)
	}
	return nil
}

// GetHistory returns recent wallet transactions for a user.
func (s *Service) GetHistory(ctx context.Context, userID string, limit int) ([]store.WalletTxRow, error) {
	return s.wallet.GetTransactions(ctx, userID, limit)
}
