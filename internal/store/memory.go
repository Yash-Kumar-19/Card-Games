package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemStore implements UserStore, WalletStore, and GameStore in-memory.
// Used for tests and development without PostgreSQL.
type MemStore struct {
	mu     sync.RWMutex
	users  map[string]*UserRow         // id -> user
	byName map[string]*UserRow         // username -> user
	txs    map[string][]WalletTxRow    // userID -> transactions
	rounds map[string]*GameRoundRow    // roundID -> round
	rPlays map[string][]RoundPlayerRow // roundID -> players
}

// NewMemStore creates an empty in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{
		users:  make(map[string]*UserRow),
		byName: make(map[string]*UserRow),
		txs:    make(map[string][]WalletTxRow),
		rounds: make(map[string]*GameRoundRow),
		rPlays: make(map[string][]RoundPlayerRow),
	}
}

// --- UserStore ---

func (m *MemStore) CreateUser(_ context.Context, username, passwordHash string, balance int64) (*UserRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byName[username]; exists {
		return nil, fmt.Errorf("username %q already taken", username)
	}
	now := time.Now()
	u := &UserRow{
		ID:        uuid.New().String(),
		Username:  username,
		Password:  passwordHash,
		Balance:   balance,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.users[u.ID] = u
	m.byName[username] = u
	return u, nil
}

// CreateUserWithID creates a user with a specific ID (for syncing with auth store).
func (m *MemStore) CreateUserWithID(_ context.Context, id, username, passwordHash string, balance int64) (*UserRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byName[username]; exists {
		return nil, fmt.Errorf("username %q already taken", username)
	}
	now := time.Now()
	u := &UserRow{
		ID:        id,
		Username:  username,
		Password:  passwordHash,
		Balance:   balance,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.users[u.ID] = u
	m.byName[username] = u
	return u, nil
}

func (m *MemStore) GetByUsername(_ context.Context, username string) (*UserRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.byName[username]
	if !ok {
		return nil, fmt.Errorf("user %q not found", username)
	}
	cp := *u
	return &cp, nil
}

func (m *MemStore) GetByID(_ context.Context, id string) (*UserRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("user %s not found", id)
	}
	cp := *u
	return &cp, nil
}

func (m *MemStore) UpdateBalance(_ context.Context, id string, newBalance int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return fmt.Errorf("user %s not found", id)
	}
	u.Balance = newBalance
	u.UpdatedAt = time.Now()
	return nil
}

// --- WalletStore ---

func (m *MemStore) DebitWithCheck(_ context.Context, userID string, amount int64, txType, reference string) (int64, error) {
	if amount <= 0 {
		return 0, errors.New("debit amount must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return 0, fmt.Errorf("user %s not found", userID)
	}
	if u.Balance < amount {
		return u.Balance, fmt.Errorf("insufficient balance: have %d, need %d", u.Balance, amount)
	}
	u.Balance -= amount
	u.UpdatedAt = time.Now()

	tx := WalletTxRow{
		ID:        uuid.New().String(),
		UserID:    userID,
		Amount:    -amount,
		Balance:   u.Balance,
		Type:      txType,
		Reference: reference,
		CreatedAt: time.Now(),
	}
	m.txs[userID] = append(m.txs[userID], tx)
	return u.Balance, nil
}

func (m *MemStore) Credit(_ context.Context, userID string, amount int64, txType, reference string) (int64, error) {
	if amount <= 0 {
		return 0, errors.New("credit amount must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return 0, fmt.Errorf("user %s not found", userID)
	}
	u.Balance += amount
	u.UpdatedAt = time.Now()

	tx := WalletTxRow{
		ID:        uuid.New().String(),
		UserID:    userID,
		Amount:    amount,
		Balance:   u.Balance,
		Type:      txType,
		Reference: reference,
		CreatedAt: time.Now(),
	}
	m.txs[userID] = append(m.txs[userID], tx)
	return u.Balance, nil
}

func (m *MemStore) RecordTransaction(_ context.Context, tx WalletTxRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs[tx.UserID] = append(m.txs[tx.UserID], tx)
	return nil
}

func (m *MemStore) GetTransactions(_ context.Context, userID string, limit int) ([]WalletTxRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	txs := m.txs[userID]
	// Return in reverse order (newest first)
	result := make([]WalletTxRow, 0, limit)
	for i := len(txs) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, txs[i])
	}
	return result, nil
}

// --- GameStore ---

func (m *MemStore) CreateRound(_ context.Context, round GameRoundRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rounds[round.ID] = &round
	return nil
}

func (m *MemStore) FinishRound(_ context.Context, roundID string, pot int64, winnerIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rounds[roundID]
	if !ok {
		return fmt.Errorf("round %s not found", roundID)
	}
	now := time.Now()
	r.Pot = pot
	r.WinnerIDs = winnerIDs
	r.FinishedAt = &now
	return nil
}

func (m *MemStore) RecordPlayerRound(_ context.Context, rp RoundPlayerRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rPlays[rp.RoundID] = append(m.rPlays[rp.RoundID], rp)
	return nil
}

func (m *MemStore) GetRoundsByUser(_ context.Context, userID string, limit int) ([]GameRoundRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []GameRoundRow
	for _, plays := range m.rPlays {
		for _, p := range plays {
			if p.UserID == userID {
				if r, ok := m.rounds[p.RoundID]; ok {
					result = append(result, *r)
				}
				break
			}
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
