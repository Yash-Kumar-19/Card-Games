package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgStore implements UserStore, WalletStore, and GameStore using PostgreSQL.
type PgStore struct {
	pool *pgxpool.Pool
}

// NewPgStore creates a new PostgreSQL store.
func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

// --- UserStore ---

func (s *PgStore) CreateUser(ctx context.Context, username, passwordHash string, balance int64) (*UserRow, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO users (username, password, balance) VALUES ($1, $2, $3)
		 RETURNING id, username, password, balance, created_at, updated_at`,
		username, passwordHash, balance,
	)
	u := &UserRow{}
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Balance, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (s *PgStore) GetByUsername(ctx context.Context, username string) (*UserRow, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, username, password, balance, created_at, updated_at FROM users WHERE username = $1`,
		username,
	)
	u := &UserRow{}
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Balance, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user %q not found", username)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *PgStore) GetByID(ctx context.Context, id string) (*UserRow, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, username, password, balance, created_at, updated_at FROM users WHERE id = $1::uuid`,
		id,
	)
	u := &UserRow{}
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Balance, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user %s not found", id)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *PgStore) UpdateBalance(ctx context.Context, id string, newBalance int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET balance = $1, updated_at = NOW() WHERE id = $2::uuid`,
		newBalance, id,
	)
	return err
}

// --- WalletStore ---

func (s *PgStore) DebitWithCheck(ctx context.Context, userID string, amount int64, txType, reference string) (int64, error) {
	if amount <= 0 {
		return 0, errors.New("debit amount must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the row and check balance
	var balance int64
	err = tx.QueryRow(ctx,
		`SELECT balance FROM users WHERE id = $1::uuid FOR UPDATE`,
		userID,
	).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("lock user: %w", err)
	}

	if balance < amount {
		return balance, fmt.Errorf("insufficient balance: have %d, need %d", balance, amount)
	}

	newBalance := balance - amount
	_, err = tx.Exec(ctx,
		`UPDATE users SET balance = $1, updated_at = NOW() WHERE id = $2::uuid`,
		newBalance, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("update balance: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO wallet_transactions (user_id, amount, balance, type, reference) VALUES ($1::uuid, $2, $3, $4, $5)`,
		userID, -amount, newBalance, txType, reference,
	)
	if err != nil {
		return 0, fmt.Errorf("record tx: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return newBalance, nil
}

func (s *PgStore) Credit(ctx context.Context, userID string, amount int64, txType, reference string) (int64, error) {
	if amount <= 0 {
		return 0, errors.New("credit amount must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var balance int64
	err = tx.QueryRow(ctx,
		`SELECT balance FROM users WHERE id = $1::uuid FOR UPDATE`,
		userID,
	).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("lock user: %w", err)
	}

	newBalance := balance + amount
	_, err = tx.Exec(ctx,
		`UPDATE users SET balance = $1, updated_at = NOW() WHERE id = $2::uuid`,
		newBalance, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("update balance: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO wallet_transactions (user_id, amount, balance, type, reference) VALUES ($1::uuid, $2, $3, $4, $5)`,
		userID, amount, newBalance, txType, reference,
	)
	if err != nil {
		return 0, fmt.Errorf("record tx: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return newBalance, nil
}

func (s *PgStore) RecordTransaction(ctx context.Context, wtx WalletTxRow) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO wallet_transactions (id, user_id, amount, balance, type, reference, created_at)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)`,
		wtx.ID, wtx.UserID, wtx.Amount, wtx.Balance, wtx.Type, wtx.Reference, wtx.CreatedAt,
	)
	return err
}

func (s *PgStore) GetTransactions(ctx context.Context, userID string, limit int) ([]WalletTxRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, amount, balance, type, reference, created_at
		 FROM wallet_transactions WHERE user_id = $1::uuid ORDER BY created_at DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []WalletTxRow
	for rows.Next() {
		var t WalletTxRow
		if err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.Balance, &t.Type, &t.Reference, &t.CreatedAt); err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

// --- GameStore ---

func (s *PgStore) CreateRound(ctx context.Context, round GameRoundRow) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO game_rounds (id, table_id, game_type, boot_amount, pot, started_at)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6)`,
		round.ID, round.TableID, round.GameType, round.BootAmount, round.Pot, round.StartedAt,
	)
	return err
}

func (s *PgStore) FinishRound(ctx context.Context, roundID string, pot int64, winnerIDs []string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx,
		`UPDATE game_rounds SET pot = $1, winner_ids = $2, finished_at = $3 WHERE id = $4::uuid`,
		pot, winnerIDs, now, roundID,
	)
	return err
}

func (s *PgStore) RecordPlayerRound(ctx context.Context, rp RoundPlayerRow) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO round_players (id, round_id, user_id, hand, result, bet_total)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6)`,
		rp.ID, rp.RoundID, rp.UserID, rp.Hand, rp.Result, rp.BetTotal,
	)
	return err
}

func (s *PgStore) GetRoundsByUser(ctx context.Context, userID string, limit int) ([]GameRoundRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT gr.id, gr.table_id, gr.game_type, gr.boot_amount, gr.pot, gr.winner_ids, gr.started_at, gr.finished_at
		 FROM game_rounds gr
		 JOIN round_players rp ON rp.round_id = gr.id
		 WHERE rp.user_id = $1::uuid
		 ORDER BY gr.started_at DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rounds []GameRoundRow
	for rows.Next() {
		var r GameRoundRow
		if err := rows.Scan(&r.ID, &r.TableID, &r.GameType, &r.BootAmount, &r.Pot, &r.WinnerIDs, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, err
		}
		rounds = append(rounds, r)
	}
	return rounds, rows.Err()
}
