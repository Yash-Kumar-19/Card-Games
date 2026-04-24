import { useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import { AuthProvider, useAuth } from './context/AuthContext'
import { GameProvider } from './context/GameContext'
import AuthPage from './pages/AuthPage'
import LobbyPage from './pages/LobbyPage'
import GamePage from './pages/GamePage'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { token } = useAuth()
  return token ? <>{children}</> : <Navigate to="/" replace />
}

function AppRoutes() {
  const { token, sessionExpired, clearSessionExpired } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    if (!sessionExpired) return
    navigate('/', { replace: true })
    const t = setTimeout(clearSessionExpired, 3000)
    return () => clearTimeout(t)
  }, [sessionExpired, clearSessionExpired, navigate])

  return (
    <>
      <Routes>
        <Route
          path="/"
          element={token ? <Navigate to="/lobby" replace /> : <AuthPage />}
        />
        <Route
          path="/lobby"
          element={
            <ProtectedRoute>
              <LobbyPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/game/:tableId"
          element={
            <ProtectedRoute>
              <GamePage />
            </ProtectedRoute>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>

      <AnimatePresence>
        {sessionExpired && (
          <motion.div
            className="error-toast"
            initial={{ opacity: 0, y: 32 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 32 }}
          >
            Session expired, please log in again
          </motion.div>
        )}
      </AnimatePresence>
    </>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <GameProvider>
          <AppRoutes />
        </GameProvider>
      </AuthProvider>
    </BrowserRouter>
  )
}
