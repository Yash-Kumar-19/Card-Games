export type EventType =
  | 'JOIN_TABLE'
  | 'LEAVE_TABLE'
  | 'START_GAME'
  | 'DEAL_CARDS'
  | 'PLAYER_ACTION'
  | 'TURN_CHANGE'
  | 'GAME_RESULT'
  | 'ERROR'
  | 'TABLE_STATE'
  | 'PLAYER_JOINED'
  | 'PLAYER_LEFT'

export type ActionType = 'blind' | 'seen' | 'call' | 'raise' | 'fold' | 'show'

export interface CardDTO {
  rank: string
  suit: string
}

export interface PlayerStateDTO {
  id: string
  name: string
  balance: number
  is_seen: boolean
  has_folded: boolean
  is_active: boolean
  card_count: number
}

export interface DealPayload {
  cards: CardDTO[]
  pot: number
  current_bet: number
  players: PlayerStateDTO[]
}

export interface TurnPayload {
  player_id: string
  current_bet: number
  pot: number
  timeout_sec: number
  players?: PlayerStateDTO[]
}

export interface ResultPayload {
  winners: string[]
  names: string[]
  pot: number
  hands: Record<string, CardDTO[]>
  balances: Record<string, number>
}

export interface TableStatePayload {
  table_id: string
  game_type: string
  state: string
  players: PlayerStateDTO[]
  pot: number
  current_bet: number
  current_turn: string
  boot_amount: number
}

export interface ServerMessage {
  type: EventType
  payload?: DealPayload | TurnPayload | ResultPayload | TableStatePayload | Record<string, unknown>
  error?: string
}

export interface ClientMessage {
  type: EventType
  table_id?: string
  action?: {
    type: ActionType
    amount?: number
  }
  create?: {
    game_type: string
    boot_amount: number
  }
}

export interface TableInfo {
  id: string
  game_type: string
  state: string
  boot_amount: number
  player_count: number
  max_players: number
}
