import { motion } from 'framer-motion'
import Card from './Card'
import type { PlayerStateDTO, CardDTO } from '../types/events'

interface PlayerSeatProps {
  player: PlayerStateDTO
  isMe: boolean
  isCurrentTurn: boolean
  myCards?: CardDTO[]
  revealedCards?: CardDTO[]
  position: { top: string; left: string }
}

export default function PlayerSeat({
  player,
  isMe,
  isCurrentTurn,
  myCards,
  revealedCards,
  position,
}: PlayerSeatProps) {
  const showFaceUp = (isMe && player.is_seen) || (revealedCards && revealedCards.length > 0)
  const displayCards = isMe ? myCards : revealedCards

  return (
    <motion.div
      className={`player-seat ${isMe ? 'me' : ''} ${isCurrentTurn ? 'active-turn' : ''} ${player.has_folded ? 'folded' : ''}`}
      style={{ top: position.top, left: position.left }}
      initial={{ opacity: 0, scale: 0.8 }}
      animate={{ opacity: 1, scale: 1 }}
    >
      {isCurrentTurn && (
        <motion.div
          className="turn-glow"
          animate={{ opacity: [0.4, 1, 0.4] }}
          transition={{ repeat: Infinity, duration: 1.2 }}
        />
      )}

      <div className="seat-cards">
        {displayCards && displayCards.length > 0 ? (
          displayCards.map((c, i) => (
            <Card key={i} card={c} faceDown={!showFaceUp} index={i} small />
          ))
        ) : (
          Array.from({ length: player.card_count || 0 }).map((_, i) => (
            <Card key={i} faceDown index={i} small />
          ))
        )}
      </div>

      <div className="seat-info">
        <div className="seat-name">
          {player.name}
          {isMe && <span className="me-badge"> (You)</span>}
        </div>
        <div className="seat-balance">₹{player.balance.toLocaleString()}</div>
        {player.has_folded && <div className="seat-status folded-tag">Folded</div>}
        {player.is_seen && !player.has_folded && <div className="seat-status seen-tag">Seen</div>}
        {!player.is_seen && !player.has_folded && player.card_count > 0 && (
          <div className="seat-status blind-tag">Blind</div>
        )}
      </div>

      {isCurrentTurn && (
        <motion.div
          className="turn-indicator"
          initial={{ scale: 0 }}
          animate={{ scale: 1 }}
          transition={{ type: 'spring' }}
        >
          ▶
        </motion.div>
      )}
    </motion.div>
  )
}
