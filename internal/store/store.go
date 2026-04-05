package store

import (
	"context"
	"time"
)

// UserRow represents a user record in the database.
type UserRow struct {
	ID        string
	Username  string
	Password  string // bcrypt hash
	Balance   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WalletTxRow represents a wallet transaction record.
type WalletTxRow struct {
	ID        string
	UserID    string
	Amount    int64  // positive = credit, negative = debit
	Balance   int64  // balance after tx
	Type      string // boot, bet, win, refund, deposit
	Reference string // e.g. table or round ID
	CreatedAt time.Time
}

// GameRoundRow represents a completed game round.
type GameRoundRow struct {
	ID         string
	TableID    string
	GameType   string
	BootAmount int64
	Pot        int64
	WinnerIDs  []string
	StartedAt  time.Time
	FinishedAt *time.Time
}

// RoundPlayerRow represents a player's participation in a round.
type RoundPlayerRow struct {
	ID       string
	RoundID  string
	UserID   string
	Hand     string // JSON
	Result   string // win, lose, fold
	BetTotal int64
}

// UserStore defines persistence operations for users.
type UserStore interface {
	CreateUser(ctx context.Context, username, passwordHash string, balance int64) (*UserRow, error)
	GetByUsername(ctx context.Context, username string) (*UserRow, error)
	GetByID(ctx context.Context, id string) (*UserRow, error)
	UpdateBalance(ctx context.Context, id string, newBalance int64) error
}

// WalletStore defines persistence operations for wallet transactions.
type WalletStore interface {
	RecordTransaction(ctx context.Context, tx WalletTxRow) error
	GetTransactions(ctx context.Context, userID string, limit int) ([]WalletTxRow, error)
	// DebitWithCheck atomically debits amount if balance >= amount. Returns new balance.
	DebitWithCheck(ctx context.Context, userID string, amount int64, txType, reference string) (int64, error)
	// Credit adds amount to user balance atomically. Returns new balance.
	Credit(ctx context.Context, userID string, amount int64, txType, reference string) (int64, error)
}

// GameStore defines persistence operations for game history.
type GameStore interface {
	CreateRound(ctx context.Context, round GameRoundRow) error
	FinishRound(ctx context.Context, roundID string, pot int64, winnerIDs []string) error
	RecordPlayerRound(ctx context.Context, rp RoundPlayerRow) error
	GetRoundsByUser(ctx context.Context, userID string, limit int) ([]GameRoundRow, error)
}
