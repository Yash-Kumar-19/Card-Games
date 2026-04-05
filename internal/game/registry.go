package game

import (
	"fmt"
	"sync"
)

// Registry holds all registered game types.
type Registry struct {
	mu    sync.RWMutex
	games map[string]Game
}

// NewRegistry creates a new empty game registry.
func NewRegistry() *Registry {
	return &Registry{
		games: make(map[string]Game),
	}
}

// Register adds a game to the registry. Returns an error if the name is already taken.
func (r *Registry) Register(g Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := g.Name()
	if _, exists := r.games[name]; exists {
		return fmt.Errorf("game %q is already registered", name)
	}
	r.games[name] = g
	return nil
}

// Get retrieves a game by name.
func (r *Registry) Get(name string) (Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.games[name]
	if !ok {
		return nil, fmt.Errorf("game %q not found", name)
	}
	return g, nil
}

// List returns the names of all registered games.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.games))
	for name := range r.games {
		names = append(names, name)
	}
	return names
}
