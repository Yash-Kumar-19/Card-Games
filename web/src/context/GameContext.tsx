import {
  createContext,
  useContext,
  useReducer,
  useRef,
  useCallback,
  useEffect,
  type ReactNode,
} from 'react'
import type {
  CardDTO,
  PlayerStateDTO,
  ResultPayload,
  TableStatePayload,
  DealPayload,
  TurnPayload,
  ServerMessage,
  ClientMessage,
  ActionType,
} from '../types/events'

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export interface GameState {
  tableId: string | null
  tableState: string // WAITING | STARTING | DEALING | BETTING | SHOWDOWN | FINISHED
  players: PlayerStateDTO[]
  myCards: CardDTO[]
  pot: number
  currentBet: number
  currentTurn: string | null
  turnTimeoutSec: number
  bootAmount: number
  result: ResultPayload | null
  revealedHands: Record<string, CardDTO[]>
  connected: boolean
  error: string | null
}

const initialState: GameState = {
  tableId: null,
  tableState: 'WAITING',
  players: [],
  myCards: [],
  pot: 0,
  currentBet: 0,
  currentTurn: null,
  turnTimeoutSec: 20,
  bootAmount: 0,
  result: null,
  revealedHands: {},
  connected: false,
  error: null,
}

// ---------------------------------------------------------------------------
// Reducer
// ---------------------------------------------------------------------------

type GameAction =
  | { type: 'CONNECTED' }
  | { type: 'DISCONNECTED' }
  | { type: 'SET_ERROR'; error: string }
  | { type: 'CLEAR_ERROR' }
  | { type: 'TABLE_STATE'; payload: TableStatePayload }
  | { type: 'PLAYER_JOINED'; player: PlayerStateDTO }
  | { type: 'PLAYER_LEFT'; playerId: string }
  | { type: 'DEAL'; payload: DealPayload }
  | { type: 'TURN'; payload: TurnPayload }
  | { type: 'RESULT'; payload: ResultPayload }
  | { type: 'LEFT_TABLE' }

function reducer(state: GameState, action: GameAction): GameState {
  switch (action.type) {
    case 'CONNECTED':
      return { ...state, connected: true, error: null }
    case 'DISCONNECTED':
      return { ...state, connected: false }
    case 'SET_ERROR':
      return { ...state, error: action.error }
    case 'CLEAR_ERROR':
      return { ...state, error: null }
    case 'TABLE_STATE': {
      const p = action.payload
      return {
        ...state,
        tableId: p.table_id,
        tableState: p.state,
        players: p.players,
        pot: p.pot,
        currentBet: p.current_bet,
        currentTurn: p.current_turn || null,
        bootAmount: p.boot_amount,
        result: null,
        revealedHands: {},
      }
    }
    case 'PLAYER_JOINED': {
      const exists = state.players.find(p => p.id === action.player.id)
      const players = exists
        ? state.players.map(p => (p.id === action.player.id ? action.player : p))
        : [...state.players, action.player]
      return { ...state, players }
    }
    case 'PLAYER_LEFT':
      return { ...state, players: state.players.filter(p => p.id !== action.playerId) }
    case 'DEAL': {
      const p = action.payload
      return {
        ...state,
        myCards: p.cards,
        pot: p.pot,
        currentBet: p.current_bet,
        players: p.players,
        tableState: 'BETTING',
        result: null,
        revealedHands: {},
      }
    }
    case 'TURN': {
      const p = action.payload
      return {
        ...state,
        currentTurn: p.player_id,
        currentBet: p.current_bet,
        pot: p.pot,
        turnTimeoutSec: p.timeout_sec,
        players: p.players ?? state.players,
      }
    }
    case 'RESULT':
      return {
        ...state,
        result: action.payload,
        revealedHands: action.payload.hands ?? {},
        tableState: 'FINISHED',
        pot: action.payload.pot,
      }
    case 'LEFT_TABLE':
      return { ...initialState, connected: state.connected }
    default:
      return state
  }
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

interface GameContextValue {
  game: GameState
  joinTable: (tableId: string) => void
  leaveTable: () => void
  startGame: () => void
  sendAction: (type: ActionType, amount?: number) => void
  connect: (token: string, userId: string) => void
  disconnect: () => void
}

const GameContext = createContext<GameContextValue | null>(null)

export function GameProvider({ children }: { children: ReactNode }) {
  const [game, dispatch] = useReducer(reducer, initialState)
  const wsRef = useRef<WebSocket | null>(null)
  const tableIdRef = useRef<string | null>(null)
  // Queued JOIN_TABLE message to send once the socket opens
  const pendingJoinRef = useRef<string | null>(null)

  // keep tableIdRef in sync
  useEffect(() => {
    tableIdRef.current = game.tableId
  }, [game.tableId])

  const send = useCallback((msg: ClientMessage) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg))
    }
  }, [])

  const connect = useCallback((token: string, _userId: string) => {
    if (wsRef.current) {
      wsRef.current.close()
    }
    const ws = new WebSocket(`/ws?token=${encodeURIComponent(token)}`)
    wsRef.current = ws

    ws.onopen = () => {
      dispatch({ type: 'CONNECTED' })
      // Flush any join that was requested before the socket was ready
      if (pendingJoinRef.current) {
        ws.send(JSON.stringify({ type: 'JOIN_TABLE', table_id: pendingJoinRef.current }))
        pendingJoinRef.current = null
      }
    }

    ws.onclose = () => dispatch({ type: 'DISCONNECTED' })

    ws.onerror = () => dispatch({ type: 'SET_ERROR', error: 'WebSocket connection error' })

    ws.onmessage = (evt) => {
      let msg: ServerMessage
      try {
        msg = JSON.parse(evt.data)
      } catch {
        return
      }

      switch (msg.type) {
        case 'TABLE_STATE':
          dispatch({ type: 'TABLE_STATE', payload: msg.payload as TableStatePayload })
          break
        case 'PLAYER_JOINED':
          dispatch({ type: 'PLAYER_JOINED', player: msg.payload as unknown as PlayerStateDTO })
          break
        case 'PLAYER_LEFT': {
          const p = msg.payload as { player_id?: string; id?: string }
          dispatch({ type: 'PLAYER_LEFT', playerId: p.player_id ?? p.id ?? '' })
          break
        }
        case 'DEAL_CARDS':
          dispatch({ type: 'DEAL', payload: msg.payload as DealPayload })
          break
        case 'TURN_CHANGE':
          dispatch({ type: 'TURN', payload: msg.payload as TurnPayload })
          break
        case 'GAME_RESULT':
          dispatch({ type: 'RESULT', payload: msg.payload as ResultPayload })
          break
        case 'ERROR':
          dispatch({ type: 'SET_ERROR', error: msg.error ?? 'Unknown error' })
          break
        default:
          break
      }
    }
  }, [])

  const disconnect = useCallback(() => {
    wsRef.current?.close()
    wsRef.current = null
    dispatch({ type: 'DISCONNECTED' })
  }, [])

  const joinTable = useCallback((tableId: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      send({ type: 'JOIN_TABLE', table_id: tableId })
    } else {
      // Socket still connecting — queue it for onopen
      pendingJoinRef.current = tableId
    }
  }, [send])

  const leaveTable = useCallback(() => {
    if (tableIdRef.current) {
      send({ type: 'LEAVE_TABLE', table_id: tableIdRef.current })
    }
    dispatch({ type: 'LEFT_TABLE' })
  }, [send])

  const startGame = useCallback(() => {
    if (tableIdRef.current) {
      send({ type: 'START_GAME', table_id: tableIdRef.current })
    }
  }, [send])

  const sendAction = useCallback((type: ActionType, amount?: number) => {
    if (tableIdRef.current) {
      send({ type: 'PLAYER_ACTION', table_id: tableIdRef.current, action: { type, amount } })
    }
  }, [send])

  return (
    <GameContext.Provider value={{ game, joinTable, leaveTable, startGame, sendAction, connect, disconnect }}>
      {children}
    </GameContext.Provider>
  )
}

export function useGame() {
  const ctx = useContext(GameContext)
  if (!ctx) throw new Error('useGame must be used within GameProvider')
  return ctx
}
