import { useState } from 'react'
import { motion } from 'framer-motion'
import type { ActionType } from '../types/events'

interface ActionButtonsProps {
  currentBet: number
  isSeen: boolean
  onAction: (type: ActionType, amount?: number) => void
}

export default function ActionButtons({ currentBet, isSeen, onAction }: ActionButtonsProps) {
  const [raiseAmount, setRaiseAmount] = useState<number>(currentBet * 2)
  const [showRaise, setShowRaise] = useState(false)

  const minRaise = isSeen ? currentBet * 2 : currentBet

  return (
    <motion.div
      className="action-panel"
      initial={{ opacity: 0, y: 24 }}
      animate={{ opacity: 1, y: 0 }}
    >
      <div className="action-info">
        <span>To play: <strong>₹{isSeen ? currentBet * 2 : currentBet}</strong></span>
        {isSeen && <span className="seen-label"> (seen = 2×)</span>}
      </div>

      <div className="action-buttons">
        {!isSeen && (
          <button
            className="btn btn-action btn-blue"
            onClick={() => onAction('seen')}
            title="Look at your cards and play as Seen"
          >
            See Cards
          </button>
        )}

        <button
          className="btn btn-action btn-green"
          onClick={() => onAction(isSeen ? 'call' : 'blind')}
          title={isSeen ? `Call ₹${currentBet * 2} (seen bet)` : `Play blind ₹${currentBet}`}
        >
          {isSeen ? `Call ₹${currentBet * 2}` : `Blind ₹${currentBet}`}
        </button>

        <button
          className="btn btn-action btn-yellow"
          onClick={() => setShowRaise(r => !r)}
        >
          Raise
        </button>

        {isSeen && (
          <button
            className="btn btn-action btn-purple"
            onClick={() => onAction('show')}
            title="Force showdown with last remaining player"
          >
            Show
          </button>
        )}

        <button
          className="btn btn-action btn-red"
          onClick={() => onAction('fold')}
        >
          Fold
        </button>
      </div>

      {showRaise && (
        <motion.div
          className="raise-panel"
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: 'auto' }}
        >
          <input
            type="number"
            className="auth-input raise-input"
            min={minRaise}
            value={raiseAmount}
            onChange={e => setRaiseAmount(Number(e.target.value))}
          />
          <button
            className="btn btn-primary"
            onClick={() => {
              onAction('raise', raiseAmount)
              setShowRaise(false)
            }}
            disabled={raiseAmount < minRaise}
          >
            Confirm Raise (₹{raiseAmount})
          </button>
        </motion.div>
      )}
    </motion.div>
  )
}
