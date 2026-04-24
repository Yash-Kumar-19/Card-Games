# Card-Games — Real-Time Multiplayer Teen Patti

A full-stack multiplayer **Teen Patti** (Three-Card Poker) platform built with Go and React. The backend runs a concurrent actor-per-table model over WebSockets; the frontend is a Vite/React TypeScript SPA.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25, `net/http`, Gorilla WebSocket |
| Auth | JWT (`golang-jwt/jwt/v5`) + bcrypt |
| Database | PostgreSQL (pgx/v5) |
| Cache / Pub-Sub | Redis (`go-redis/v9`) |
| Frontend | React 18, TypeScript, Vite |
| IDs | UUIDs (`google/uuid`) |

---

## Architecture

```
┌──────────────────────────────────────────────────┐
│                   HTTP + WS Server                │
│                                                    │
│  POST /api/register   POST /api/login             │
│  GET  /api/tables     POST /api/tables            │
│  GET  /api/wallet/balance  GET /api/wallet/transactions │
│  GET  /ws  (WebSocket, JWT via query param)       │
└──────────────┬───────────────────────────────────┘
               │
       ┌───────▼────────┐
       │   WebSocket Hub │  — one goroutine per client connection
       │   (ws.Hub)      │  — OnMessage / OnReconnect / OnDisconnect hooks
       └───────┬────────┘
               │ dispatches ClientMessage
       ┌───────▼────────┐
       │  Lobby          │  — registry of active TableActors
       └───────┬────────┘
               │
       ┌───────▼────────────────────────┐
       │  TableActor (1 goroutine/table) │  — actor model, no locks needed
       │  engine.Table  ←  game.Game    │  — pluggable game interface
       │  WalletHook                    │
       └────────────────────────────────┘
```

Each `TableActor` owns a single goroutine that processes all events for that table sequentially, eliminating the need for mutexes on game state.

---

## Project Structure

```
.
├── cmd/server/main.go          # Entry point — wires all services, HTTP routes
├── config/config.go            # Env-var based configuration
├── internal/
│   ├── auth/                   # User store + JWT service
│   ├── cache/redis.go          # Redis client wrapper
│   ├── engine/
│   │   ├── actor.go            # TableActor — goroutine per table
│   │   ├── table.go            # Table state machine (WAITING → FINISHED)
│   │   ├── deck.go             # 52-card deck with crypto/rand shuffle
│   │   └── player.go           # Player model
│   ├── game/
│   │   ├── game.go             # Game interface (DealCards, ApplyAction, etc.)
│   │   ├── registry.go         # Registry of available game types
│   │   └── games/teenpatti/    # Teen Patti implementation
│   ├── lobby/lobby.go          # Table lifecycle + player-table mapping
│   ├── model/model.go          # Shared domain models
│   ├── store/
│   │   ├── store.go            # Store interface
│   │   ├── memory.go           # In-memory store (default / dev)
│   │   └── postgres.go         # PostgreSQL store (production)
│   ├── wallet/wallet.go        # Boot collection + winnings credit
│   └── ws/
│       ├── hub.go              # WebSocket connection hub
│       └── events.go           # Event type constants
├── migrations/
│   └── 001_initial_schema.up.sql   # Users, wallet_transactions, game_rounds
├── tests/integration_test.go   # Integration test suite
└── web/                        # Vite + React + TypeScript frontend
```

---

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 20+ (for the frontend)
- PostgreSQL 14+ (optional — defaults to in-memory store)
- Redis 7+ (optional — for caching and pub-sub)

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `JWT_SECRET` | `change-me-in-production` | Secret for signing JWTs |
| `JWT_EXPIRY` | `86400` (24 h, in seconds) | Token lifetime |
| `DATABASE_URL` | `postgres://cardgames:cardgames@localhost:5432/cardgames?sslmode=disable` | PostgreSQL DSN |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis URL |
| `TURN_TIMEOUT` | `20` (seconds) | Auto-fold timeout per turn |

### Run the backend

```bash
# Clone and enter the repo
git clone <repo-url>
cd card-games

# Install Go dependencies
go mod download

# (Optional) run migrations against PostgreSQL
psql "$DATABASE_URL" -f migrations/001_initial_schema.up.sql

# Start the server
go run ./cmd/server
```

### Run the frontend

```bash
cd web
npm install
npm run dev      # development server on http://localhost:5173
npm run build    # production build → web/dist/
```

The backend serves the built frontend from `web/dist/` in production.

---

## API Reference

All authenticated endpoints require `Authorization: Bearer <token>`.

### Auth

| Method | Path | Body | Response |
|---|---|---|---|
| `POST` | `/api/register` | `{ "username": "", "password": "" }` | `{ "token", "user_id", "balance" }` |
| `POST` | `/api/login` | `{ "username": "", "password": "" }` | `{ "token", "user_id" }` |

### Lobby

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/tables` | Yes | List all active tables |
| `POST` | `/api/tables` | Yes | Create a table — body: `{ "game_type": "teen_patti", "boot_amount": 10 }` |

### Wallet

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/wallet/balance` | Yes | Current balance |
| `GET` | `/api/wallet/transactions` | Yes | Last 50 transactions |

### WebSocket

Connect at `GET /ws?token=<jwt>`.

---

## WebSocket Events

### Client → Server

```json
{ "type": "join_table",    "table_id": "<id>" }
{ "type": "leave_table",   "table_id": "<id>" }
{ "type": "start_game",    "table_id": "<id>" }
{ "type": "player_action", "table_id": "<id>", "action": { "type": "blind|seen|call|raise|fold|show", "amount": 0 } }
```

### Server → Client

```json
{ "type": "deal_cards",    "payload": { ... } }
{ "type": "turn_change",   "payload": { ... } }
{ "type": "game_result",   "payload": { "winners": ["player_id"] } }
{ "type": "error",         "error": "message" }
```

---

## Teen Patti Rules

Teen Patti ("Three Cards") is a South Asian betting card game played with a standard 52-card deck.

### Hand Rankings (high → low)

| Rank | Name | Description |
|---|---|---|
| 1 | Trail / Set | Three cards of the same rank (e.g., A-A-A) |
| 2 | Pure Sequence | Three consecutive cards of the same suit |
| 3 | Sequence / Run | Three consecutive cards (mixed suits) |
| 4 | Color / Flush | Three cards of the same suit (not in sequence) |
| 5 | Pair | Two cards of the same rank |
| 6 | High Card | None of the above; highest card wins |

### Game Flow

1. **Boot** — every player antes the boot amount before cards are dealt.
2. **Deal** — each player receives 3 cards face-down; dealer rotates each round.
3. **Betting rounds** — starting left of dealer, each player in turn must:
   - **Blind** — bet without looking at their cards (bet = current stake).
   - **Seen** — look at cards and bet (minimum = 2× current blind stake).
   - **Call** — match the current bet.
   - **Raise** — increase the current bet.
   - **Fold** — surrender hand and forfeit bets.
   - **Show** — force an immediate showdown (only when two players remain).
4. **Showdown** — remaining players reveal cards; best hand wins the pot.

### Minimum / Maximum Players

| Setting | Value |
|---|---|
| Minimum players | 3 |
| Maximum players | 6 |

---

## Database Schema

Three tables are created by `migrations/001_initial_schema.up.sql`:

- **`users`** — account credentials and current balance (bcrypt password, UUID primary key).
- **`wallet_transactions`** — immutable ledger; each row records a debit or credit with running balance and type (`boot`, `bet`, `win`, `refund`, `deposit`).
- **`game_rounds`** + **`round_players`** — full game history with hand results.

The server defaults to the in-memory store so you can run without PostgreSQL during development. Swap `store.NewMemStore()` for `store.NewPgStore(cfg.DatabaseURL)` in `cmd/server/main.go` to enable persistence.

---

## Running Tests

```bash
# Unit tests
go test ./internal/...

# Integration tests (requires a running server or test fixtures)
go test ./tests/...
```

---

## Roadmap

- [ ] Private / password-protected tables
- [ ] In-game chat
- [ ] Spectator mode
- [ ] Leaderboard
- [ ] Tournament brackets
- [ ] AI bots
- [ ] Provably-fair shuffle verification
- [ ] Docker Compose for one-command local setup
