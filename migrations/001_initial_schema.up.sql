-- 001_initial_schema.up.sql
-- Users, wallet transactions, and game history tables.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username   VARCHAR(50) UNIQUE NOT NULL,
    password   VARCHAR(255) NOT NULL,  -- bcrypt hash
    balance    BIGINT NOT NULL DEFAULT 10000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_username ON users(username);

-- Wallet transactions (immutable ledger)
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id),
    amount      BIGINT NOT NULL,        -- positive = credit, negative = debit
    balance     BIGINT NOT NULL,        -- balance after this transaction
    type        VARCHAR(30) NOT NULL,   -- 'boot', 'bet', 'win', 'refund', 'deposit'
    reference   VARCHAR(255),           -- e.g., table ID or round ID
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wallet_tx_user ON wallet_transactions(user_id);
CREATE INDEX idx_wallet_tx_created ON wallet_transactions(created_at);

-- Game history
CREATE TABLE IF NOT EXISTS game_rounds (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    table_id    VARCHAR(255) NOT NULL,
    game_type   VARCHAR(50) NOT NULL,
    boot_amount BIGINT NOT NULL,
    pot         BIGINT NOT NULL DEFAULT 0,
    winner_ids  TEXT[],                  -- array of winner user IDs
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_game_rounds_table ON game_rounds(table_id);

-- Player participation in a round
CREATE TABLE IF NOT EXISTS round_players (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    round_id  UUID NOT NULL REFERENCES game_rounds(id),
    user_id   UUID NOT NULL REFERENCES users(id),
    hand      TEXT,           -- JSON serialized cards (revealed at showdown)
    result    VARCHAR(20),    -- 'win', 'lose', 'fold'
    bet_total BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_round_players_round ON round_players(round_id);
CREATE INDEX idx_round_players_user ON round_players(user_id);
