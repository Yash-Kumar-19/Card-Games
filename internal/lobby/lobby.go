package lobby

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nakad/cardgames/internal/engine"
	"github.com/nakad/cardgames/internal/game"
)

// TableInfo is the public listing info for a table.
type TableInfo struct {
	ID          string `json:"id"`
	GameType    string `json:"game_type"`
	BootAmount  int64  `json:"boot_amount"`
	PlayerCount int    `json:"player_count"`
	MaxPlayers  int    `json:"max_players"`
	State       string `json:"state"`
}

// Lobby manages table creation, listing, and lifecycle.
type Lobby struct {
	mu          sync.RWMutex
	tables      map[string]*engine.TableActor
	registry    *game.Registry
	turnTimeout time.Duration
}

// NewLobby creates a new lobby.
func NewLobby(registry *game.Registry, turnTimeout time.Duration) *Lobby {
	return &Lobby{
		tables:      make(map[string]*engine.TableActor),
		registry:    registry,
		turnTimeout: turnTimeout,
	}
}

// CreateTable creates a new table and returns its actor.
func (l *Lobby) CreateTable(gameType string, bootAmount int64) (*engine.TableActor, error) {
	g, err := l.registry.Get(gameType)
	if err != nil {
		return nil, fmt.Errorf("unknown game type: %s", gameType)
	}

	id := uuid.New().String()[:8]
	table := engine.NewTable(id, g, bootAmount)
	actor := engine.NewTableActor(table, l.turnTimeout)

	l.mu.Lock()
	l.tables[id] = actor
	l.mu.Unlock()

	return actor, nil
}

// GetTable returns a table actor by ID.
func (l *Lobby) GetTable(id string) *engine.TableActor {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tables[id]
}

// RemoveTable removes and stops a table.
func (l *Lobby) RemoveTable(id string) {
	l.mu.Lock()
	actor, ok := l.tables[id]
	if ok {
		delete(l.tables, id)
	}
	l.mu.Unlock()
	if ok {
		actor.Stop()
	}
}

// ListTables returns info about all active tables.
func (l *Lobby) ListTables() []TableInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()

	infos := make([]TableInfo, 0, len(l.tables))
	for _, actor := range l.tables {
		t := actor.Table
		infos = append(infos, TableInfo{
			ID:          t.ID,
			GameType:    t.Game.Name(),
			BootAmount:  t.BootAmount,
			PlayerCount: len(t.Players),
			MaxPlayers:  t.Game.MaxPlayers(),
			State:       t.State.String(),
		})
	}
	return infos
}

// ListTablesByGame returns tables filtered by game type.
func (l *Lobby) ListTablesByGame(gameType string) []TableInfo {
	all := l.ListTables()
	filtered := make([]TableInfo, 0)
	for _, info := range all {
		if info.GameType == gameType {
			filtered = append(filtered, info)
		}
	}
	return filtered
}

// PlayerTableMap tracks which table each player is at.
type PlayerTableMap struct {
	mu    sync.RWMutex
	table map[string]string // playerID -> tableID
}

// NewPlayerTableMap creates a new player-table mapping.
func NewPlayerTableMap() *PlayerTableMap {
	return &PlayerTableMap{
		table: make(map[string]string),
	}
}

func (m *PlayerTableMap) Set(playerID, tableID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.table[playerID] = tableID
}

func (m *PlayerTableMap) Get(playerID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.table[playerID]
}

func (m *PlayerTableMap) Delete(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.table, playerID)
}
