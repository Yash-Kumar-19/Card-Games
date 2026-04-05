package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache provides game state caching and pub/sub for multi-instance coordination.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache from a connection URL.
func NewRedisCache(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisCache{client: client}, nil
}

// Close shuts down the Redis connection.
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// --- Game State Caching ---

const gameStatePrefix = "game:state:"
const gameStateTTL = 2 * time.Hour

// GameSnapshot is a serializable snapshot of a table's game state.
type GameSnapshot struct {
	TableID     string                    `json:"table_id"`
	GameType    string                    `json:"game_type"`
	State       string                    `json:"state"`
	Pot         int64                     `json:"pot"`
	CurrentBet  int64                     `json:"current_bet"`
	CurrentTurn int                       `json:"current_turn"`
	DealerIndex int                       `json:"dealer_index"`
	BootAmount  int64                     `json:"boot_amount"`
	Players     []PlayerSnapshot          `json:"players"`
	Hands       map[string][]CardSnapshot `json:"hands"`
	Bets        map[string]int64          `json:"bets"`
	ActiveIDs   []string                  `json:"active_ids"`
}

// PlayerSnapshot is a serializable player state.
type PlayerSnapshot struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Balance   int64  `json:"balance"`
	IsSeen    bool   `json:"is_seen"`
	HasFolded bool   `json:"has_folded"`
	IsActive  bool   `json:"is_active"`
}

// CardSnapshot is a serializable card.
type CardSnapshot struct {
	Rank string `json:"rank"`
	Suit string `json:"suit"`
}

// SaveGameState stores a game state snapshot in Redis.
func (c *RedisCache) SaveGameState(ctx context.Context, snapshot *GameSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal game state: %w", err)
	}
	key := gameStatePrefix + snapshot.TableID
	return c.client.Set(ctx, key, data, gameStateTTL).Err()
}

// GetGameState retrieves a game state snapshot from Redis.
func (c *RedisCache) GetGameState(ctx context.Context, tableID string) (*GameSnapshot, error) {
	key := gameStatePrefix + tableID
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snapshot GameSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("unmarshal game state: %w", err)
	}
	return &snapshot, nil
}

// DeleteGameState removes a game state from Redis.
func (c *RedisCache) DeleteGameState(ctx context.Context, tableID string) error {
	return c.client.Del(ctx, gameStatePrefix+tableID).Err()
}

// --- Player Session Tracking ---

const playerSessionPrefix = "player:session:"
const playerSessionTTL = 24 * time.Hour

// PlayerSession tracks which table a player is at.
type PlayerSession struct {
	UserID  string `json:"user_id"`
	TableID string `json:"table_id"`
}

// SetPlayerSession records a player's active table.
func (c *RedisCache) SetPlayerSession(ctx context.Context, userID, tableID string) error {
	data, _ := json.Marshal(PlayerSession{UserID: userID, TableID: tableID})
	return c.client.Set(ctx, playerSessionPrefix+userID, data, playerSessionTTL).Err()
}

// GetPlayerSession returns the table a player was last at.
func (c *RedisCache) GetPlayerSession(ctx context.Context, userID string) (*PlayerSession, error) {
	data, err := c.client.Get(ctx, playerSessionPrefix+userID).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var session PlayerSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// DeletePlayerSession removes a player session.
func (c *RedisCache) DeletePlayerSession(ctx context.Context, userID string) error {
	return c.client.Del(ctx, playerSessionPrefix+userID).Err()
}

// --- Pub/Sub for Multi-Instance ---

const pubsubChannel = "cardgames:events"

// PublishEvent publishes an event to other server instances.
func (c *RedisCache) PublishEvent(ctx context.Context, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.client.Publish(ctx, pubsubChannel, data).Err()
}

// Subscribe returns a channel that receives events from other instances.
func (c *RedisCache) Subscribe(ctx context.Context) (<-chan string, func()) {
	sub := c.client.Subscribe(ctx, pubsubChannel)
	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		for msg := range sub.Channel() {
			select {
			case ch <- msg.Payload:
			default:
				// drop if consumer is slow
			}
		}
	}()
	cancel := func() { sub.Close() }
	return ch, cancel
}
