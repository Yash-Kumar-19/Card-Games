import { useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../context/GameContext'
import { useAuth } from '../context/AuthContext'
import PlayerSeat from '../components/PlayerSeat'
import ActionButtons from '../components/ActionButtons'
import TurnTimer from '../components/TurnTimer'
import GameResult from '../components/GameResult'
import type { PlayerStateDTO } from '../types/events'

// Seat positions around an ellipse (top%, left%) for up to 6 players.
// Index 0 = bottom-center (local player). Others spread clockwise.
const SEAT_POSITIONS: Array<{ top: string; left: string }> = [
  { top: '74%', left: '42%' }, // bottom (me)
  { top: '60%', left: '82%' }, // bottom-right
  { top: '20%', left: '76%' }, // top-right
  { top: '8%',  left: '40%' }, // top-center
  { top: '20%', left: '4%'  }, // top-left
  { top: '60%', left: '0%'  }, // bottom-left
]

function getSeatPositions(players: PlayerStateDTO[], myId: string | null) {
  const myIndex = players.findIndex(p => p.id === myId)
  return players.map((p, i) => {
    const offset = myIndex >= 0 ? (i - myIndex + players.length) % players.length : i
    return { player: p, pos: SEAT_POSITIONS[offset] ?? SEAT_POSITIONS[0] }
  })
}

export default function GamePage() {
  const { tableId } = useParams<{ tableId: string }>()
  const navigate = useNavigate()
  const { game, joinTable, leaveTable, startGame, sendAction, connect } = useGame()
  const { userId, username, token } = useAuth()

  // Connect WebSocket if not already open (handles page refresh / direct navigation)
  useEffect(() => {
    if (token && !game.connected) {
      connect(token, '')
    }
  }, [token, connect]) // eslint-disable-line react-hooks/exhaustive-deps

  // Join table when we land here (handles direct navigation / page refresh)
  useEffect(() => {
    if (tableId && !game.tableId) {
      joinTable(tableId)
    }
  }, [tableId, game.tableId, joinTable])

  const isMyTurn = game.currentTurn === userId
  const myPlayer = game.players.find(p => p.id === userId)
  const seated = getSeatPositions(game.players, userId)

  const handleLeave = () => {
    leaveTable()
    navigate('/lobby')
  }

  const handlePlayAgain = () => {
    startGame()
  }

  const handleLobby = () => {
    leaveTable()
    navigate('/lobby')
  }

  return (
    <div className="game-page">
      {/* Top bar */}
      <header className="game-header">
        <button className="btn btn-ghost" onClick={handleLeave}>← Leave</button>
        <div className="game-header-center">
          <span className="game-title">Teen Patti</span>
          <span className={`game-state-badge state-${game.tableState.toLowerCase()}`}>
            {game.tableState}
          </span>
        </div>
        <div className="game-header-right">
          <span className="header-username">{username}</span>
        </div>
      </header>

      {/* Table */}
      <div className="table-container">
        <div className="felt-table">
          {/* Center info */}
          <div className="table-center">
            <motion.div
              className="pot-display"
              animate={{ scale: game.pot > 0 ? [1, 1.08, 1] : 1 }}
              transition={{ duration: 0.3 }}
            >
              <span className="pot-label">POT</span>
              <span className="pot-amount">₹{game.pot.toLocaleString()}</span>
            </motion.div>

            {game.currentBet > 0 && (
              <div className="current-bet">
                Bet: ₹{game.currentBet}
              </div>
            )}

            {isMyTurn && game.tableState === 'BETTING' && (
              <TurnTimer seconds={game.turnTimeoutSec} isMyTurn={isMyTurn} />
            )}
          </div>

          {/* Player seats */}
          {seated.map(({ player, pos }) => (
            <PlayerSeat
              key={player.id}
              player={player}
              isMe={player.id === userId}
              isCurrentTurn={game.currentTurn === player.id}
              myCards={player.id === userId ? game.myCards : undefined}
              revealedCards={game.revealedHands[player.id]}
              position={pos}
            />
          ))}

          {/* Waiting state overlay */}
          {game.tableState === 'WAITING' && (
            <motion.div
              className="waiting-overlay"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
            >
              <p className="waiting-text">
                {game.players.length < 3
                  ? `Waiting for players… (${game.players.length}/3 min)`
                  : 'Ready to start!'}
              </p>
              {game.players.length >= 3 && (
                <button className="btn btn-primary" onClick={startGame}>
                  Start Game
                </button>
              )}
            </motion.div>
          )}
        </div>
      </div>

      {/* Action panel */}
      <AnimatePresence>
        {isMyTurn && game.tableState === 'BETTING' && myPlayer && !myPlayer.has_folded && (
          <ActionButtons
            currentBet={game.currentBet}
            isSeen={myPlayer.is_seen}
            onAction={sendAction}
          />
        )}
      </AnimatePresence>

      {/* Error toast */}
      <AnimatePresence>
        {game.error && (
          <motion.div
            className="error-toast"
            initial={{ opacity: 0, y: 32 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 32 }}
          >
            {game.error}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Game result */}
      <AnimatePresence>
        {game.result && (
          <GameResult
            result={game.result}
            myUserId={userId}
            players={game.players}
            onPlayAgain={handlePlayAgain}
            onLobby={handleLobby}
          />
        )}
      </AnimatePresence>
    </div>
  )
}
