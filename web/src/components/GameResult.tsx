import { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import Card from './Card'
import type { ResultPayload, PlayerStateDTO } from '../types/events'

const AUTO_START_SEC = 10

interface GameResultProps {
  result: ResultPayload
  myUserId: string | null
  players: PlayerStateDTO[]
  onPlayAgain: () => void
  onLobby: () => void
}

export default function GameResult({ result, myUserId, players, onPlayAgain, onLobby }: GameResultProps) {
  const [countdown, setCountdown] = useState(AUTO_START_SEC)
  const isWinner = result.winners.some(id => id === myUserId)

  useEffect(() => {
    setCountdown(AUTO_START_SEC)
    const interval = setInterval(() => {
      setCountdown(s => {
        if (s <= 1) {
          clearInterval(interval)
          onPlayAgain()
          return 0
        }
        return s - 1
      })
    }, 1000)
    return () => clearInterval(interval)
  }, [onPlayAgain])

  const nameOf = (playerId: string): string => {
    const winnerIdx = result.winners.indexOf(playerId)
    if (winnerIdx >= 0) return result.names[winnerIdx]
    return players.find(p => p.id === playerId)?.name ?? playerId
  }

  return (
    <motion.div
      className="result-overlay"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
    >
      <motion.div
        className="result-card"
        initial={{ scale: 0.6, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        transition={{ type: 'spring', stiffness: 200, damping: 18 }}
      >
        <motion.div
          className={`result-banner ${isWinner ? 'winner' : 'loser'}`}
          animate={isWinner ? { scale: [1, 1.05, 1] } : {}}
          transition={{ repeat: isWinner ? Infinity : 0, duration: 1.4 }}
        >
          {isWinner ? '🏆 You Win!' : '😔 Better luck next time'}
        </motion.div>

        <div className="result-pot">Pot: ₹{result.pot.toLocaleString()}</div>

        <div className="result-winners">
          {result.winners.map((winnerId, i) => (
            <motion.div
              key={winnerId}
              className="winner-row"
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: i * 0.15 }}
            >
              <span className="winner-name">{result.names[i]}</span>
              <div className="winner-cards">
                {result.hands[winnerId]?.map((c, j) => (
                  <Card key={j} card={c} index={j} small />
                ))}
              </div>
            </motion.div>
          ))}
        </div>

        {Object.keys(result.hands).length > 0 && (
          <div className="all-hands">
            <h4>All Hands</h4>
            {Object.entries(result.hands).map(([playerId, cards]) => (
              <div key={playerId} className="hand-row">
                <span className="hand-player-id">{nameOf(playerId)}</span>
                <div className="hand-cards">
                  {cards.map((c, i) => (
                    <Card key={i} card={c} index={i} small />
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}

        <div className="result-actions">
          <button className="btn btn-primary" onClick={onPlayAgain}>
            Play Again ({countdown}s)
          </button>
          <button className="btn btn-ghost" onClick={onLobby}>Lobby</button>
        </div>
      </motion.div>
    </motion.div>
  )
}
