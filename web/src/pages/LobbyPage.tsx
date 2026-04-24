import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import { useAuth } from '../context/AuthContext'
import { useGame } from '../context/GameContext'
import type { TableInfo } from '../types/events'

export default function LobbyPage() {
  const { token, username, balance, logout, refreshBalance } = useAuth()
  const { connect, joinTable, game } = useGame()
  const navigate = useNavigate()

  const [tables, setTables] = useState<TableInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [bootAmount, setBootAmount] = useState(10)
  const [error, setError] = useState('')

  // Connect WebSocket on mount
  useEffect(() => {
    if (token) {
      connect(token, '')
    }
  }, [token, connect])

  const fetchTables = useCallback(async () => {
    if (!token) return
    try {
      const res = await fetch('/api/tables', {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await res.json()
      setTables(Array.isArray(data) ? data : [])
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    fetchTables()
    const interval = setInterval(fetchTables, 3000)
    return () => clearInterval(interval)
  }, [fetchTables])

  useEffect(() => {
    refreshBalance()
  }, [refreshBalance])

  // Navigate to game once we have a table state
  useEffect(() => {
    if (game.tableId) {
      navigate(`/game/${game.tableId}`)
    }
  }, [game.tableId, navigate])

  const handleJoin = (tableId: string) => {
    joinTable(tableId)
  }

  const handleCreate = async () => {
    if (!token) return
    setCreating(true)
    setError('')
    try {
      const res = await fetch('/api/tables', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ game_type: 'teen_patti', boot_amount: bootAmount }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? 'Failed to create table')
      setShowCreate(false)
      // Join the table we just created
      joinTable(data.table_id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error creating table')
    } finally {
      setCreating(false)
    }
  }

  const stateLabel: Record<string, string> = {
    WAITING: 'Waiting',
    STARTING: 'Starting',
    DEALING: 'In Progress',
    BETTING: 'In Progress',
    SHOWDOWN: 'Showdown',
    FINISHED: 'Finished',
  }

  const stateColor: Record<string, string> = {
    WAITING: 'var(--green)',
    STARTING: 'var(--yellow)',
    DEALING: 'var(--orange)',
    BETTING: 'var(--orange)',
    SHOWDOWN: 'var(--red)',
    FINISHED: 'var(--muted)',
  }

  return (
    <div className="lobby-page">
      <header className="lobby-header">
        <div className="lobby-brand">
          <span className="suit red">♥</span>
          <span className="lobby-title">Teen Patti</span>
        </div>
        <div className="lobby-user">
          <span className="lobby-balance">₹{balance.toLocaleString()}</span>
          <span className="lobby-username">{username}</span>
          <button className="btn btn-ghost" onClick={logout}>Sign Out</button>
        </div>
      </header>

      <main className="lobby-main">
        <div className="lobby-toolbar">
          <h2>Game Tables</h2>
          <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
            + New Table
          </button>
        </div>

        <AnimatePresence>
          {showCreate && (
            <motion.div
              className="create-panel"
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
            >
              <h3>Create Table</h3>
              <div className="create-row">
                <label>Boot Amount (₹)</label>
                <input
                  type="number"
                  className="auth-input create-input"
                  min={1}
                  value={bootAmount}
                  onChange={e => setBootAmount(Number(e.target.value))}
                />
              </div>
              {error && <p className="auth-error">{error}</p>}
              <div className="create-actions">
                <button className="btn btn-ghost" onClick={() => setShowCreate(false)}>Cancel</button>
                <button className="btn btn-primary" onClick={handleCreate} disabled={creating}>
                  {creating ? 'Creating…' : 'Create & Join'}
                </button>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {loading ? (
          <div className="lobby-empty">Loading tables…</div>
        ) : tables.length === 0 ? (
          <div className="lobby-empty">
            <p>No tables yet.</p>
            <p>Create one to start playing!</p>
          </div>
        ) : (
          <div className="table-grid">
            {tables.map((table, i) => (
              <motion.div
                key={table.id}
                className="table-card"
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.05 }}
              >
                <div className="table-card-header">
                  <span
                    className="table-state-badge"
                    style={{ color: stateColor[table.state] ?? 'var(--muted)' }}
                  >
                    {stateLabel[table.state] ?? table.state}
                  </span>
                  <span className="table-game-type">Teen Patti</span>
                </div>
                <div className="table-card-body">
                  <div className="table-stat">
                    <span className="table-stat-label">Boot</span>
                    <span className="table-stat-value">₹{table.boot_amount}</span>
                  </div>
                  <div className="table-stat">
                    <span className="table-stat-label">Players</span>
                    <span className="table-stat-value">
                      {table.player_count ?? 0}/{table.max_players ?? 6}
                    </span>
                  </div>
                </div>
                <button
                  className="btn btn-primary table-join-btn"
                  onClick={() => handleJoin(table.id)}
                  disabled={table.state !== 'WAITING'}
                >
                  {table.state === 'WAITING' ? 'Join' : 'Spectate (soon)'}
                </button>
              </motion.div>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
