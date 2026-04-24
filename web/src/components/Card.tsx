import { motion } from 'framer-motion'
import type { CardDTO } from '../types/events'

const SUIT_SYMBOL: Record<string, string> = {
  Spades: '♠',
  Hearts: '♥',
  Diamonds: '♦',
  Clubs: '♣',
}

const SUIT_COLOR: Record<string, string> = {
  Spades: '#1a1a2e',
  Hearts: '#c0392b',
  Diamonds: '#c0392b',
  Clubs: '#1a1a2e',
}

interface CardProps {
  card?: CardDTO
  faceDown?: boolean
  index?: number
  small?: boolean
}

export default function Card({ card, faceDown = false, index = 0, small = false }: CardProps) {
  const size = small
    ? { width: 44, height: 62, fontSize: 13, suitSize: 11 }
    : { width: 64, height: 90, fontSize: 18, suitSize: 16 }

  return (
    <motion.div
      className={`playing-card ${faceDown ? 'face-down' : ''}`}
      style={{ width: size.width, height: size.height }}
      initial={{ opacity: 0, y: -40, rotate: -15 }}
      animate={{ opacity: 1, y: 0, rotate: 0 }}
      transition={{ delay: index * 0.12, type: 'spring', stiffness: 260, damping: 20 }}
    >
      {faceDown || !card ? (
        <div className="card-back">
          <div className="card-back-pattern" />
        </div>
      ) : (
        <div
          className="card-face"
          style={{ color: SUIT_COLOR[card.suit] ?? '#1a1a2e' }}
        >
          <div className="card-corner card-corner-tl" style={{ fontSize: size.fontSize }}>
            <div className="card-rank">{card.rank}</div>
            <div className="card-suit-small" style={{ fontSize: size.suitSize }}>
              {SUIT_SYMBOL[card.suit] ?? card.suit}
            </div>
          </div>
          <div className="card-center-suit" style={{ fontSize: size.width * 0.55 }}>
            {SUIT_SYMBOL[card.suit] ?? card.suit}
          </div>
          <div className="card-corner card-corner-br" style={{ fontSize: size.fontSize }}>
            <div className="card-rank">{card.rank}</div>
            <div className="card-suit-small" style={{ fontSize: size.suitSize }}>
              {SUIT_SYMBOL[card.suit] ?? card.suit}
            </div>
          </div>
        </div>
      )}
    </motion.div>
  )
}
