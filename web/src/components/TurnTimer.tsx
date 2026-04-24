import { useEffect, useState } from 'react'
import { motion } from 'framer-motion'

interface TurnTimerProps {
  seconds: number
  isMyTurn: boolean
}

export default function TurnTimer({ seconds, isMyTurn }: TurnTimerProps) {
  const [remaining, setRemaining] = useState(seconds)

  useEffect(() => {
    setRemaining(seconds)
    const interval = setInterval(() => {
      setRemaining(r => Math.max(0, r - 1))
    }, 1000)
    return () => clearInterval(interval)
  }, [seconds])

  const pct = seconds > 0 ? remaining / seconds : 0
  const radius = 20
  const circumference = 2 * Math.PI * radius
  const strokeDashoffset = circumference * (1 - pct)

  const color = remaining <= 5 ? '#e74c3c' : remaining <= 10 ? '#f39c12' : '#2ecc71'

  return (
    <motion.div
      className={`turn-timer ${isMyTurn ? 'my-turn' : ''}`}
      initial={{ scale: 0 }}
      animate={{ scale: 1 }}
    >
      <svg width={52} height={52} viewBox="0 0 52 52">
        <circle cx={26} cy={26} r={radius} fill="none" stroke="rgba(255,255,255,0.15)" strokeWidth={4} />
        <motion.circle
          cx={26}
          cy={26}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={4}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={strokeDashoffset}
          transform="rotate(-90 26 26)"
          animate={{ strokeDashoffset, stroke: color }}
          transition={{ duration: 0.5 }}
        />
      </svg>
      <span className="timer-number" style={{ color }}>
        {remaining}
      </span>
    </motion.div>
  )
}
