import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useRef,
  type ReactNode,
} from 'react'

interface AuthState {
  token: string | null
  userId: string | null
  username: string | null
  balance: number
}

interface AuthContextValue extends AuthState {
  login: (username: string, password: string) => Promise<void>
  register: (username: string, password: string) => Promise<void>
  logout: () => void
  refreshBalance: () => Promise<void>
  sessionExpired: boolean
  clearSessionExpired: () => void
  authFetch: (url: string, options?: RequestInit) => Promise<Response>
}

const AuthContext = createContext<AuthContextValue | null>(null)
const STORAGE_KEY = 'cg_auth'

function loadStored(): AuthState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw)
  } catch { /* ignore */ }
  return { token: null, userId: null, username: null, balance: 0 }
}

// Decode JWT payload, return exp as milliseconds or null if absent / malformed.
function getTokenExpiry(token: string): number | null {
  try {
    const b64 = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4)
    const payload = JSON.parse(atob(padded))
    return typeof payload.exp === 'number' ? payload.exp * 1000 : null
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>(loadStored)
  const [sessionExpired, setSessionExpired] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const persist = (s: AuthState) => {
    setState(s)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(s))
  }

  const logout = useCallback(() => {
    localStorage.removeItem(STORAGE_KEY)
    setState({ token: null, userId: null, username: null, balance: 0 })
  }, [])

  const forceLogout = useCallback(() => {
    setSessionExpired(true)
    localStorage.removeItem(STORAGE_KEY)
    setState({ token: null, userId: null, username: null, balance: 0 })
  }, [])

  const clearSessionExpired = useCallback(() => setSessionExpired(false), [])

  // --- Trigger 1: JWT clock expiry ---
  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    if (!state.token) return

    const checkExpiry = () => {
      const exp = getTokenExpiry(state.token!)
      if (exp === null) return
      const remaining = exp - Date.now()
      if (remaining <= 0) {
        forceLogout()
      } else {
        if (timerRef.current) clearTimeout(timerRef.current)
        timerRef.current = setTimeout(forceLogout, remaining)
      }
    }

    checkExpiry()

    const onVisible = () => {
      if (document.visibilityState === 'visible') checkExpiry()
    }
    document.addEventListener('visibilitychange', onVisible)

    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [state.token, forceLogout])

  // --- Trigger 2: Backend restart probe (runs once on mount) ---
  useEffect(() => {
    const { token } = loadStored()
    if (!token) return
    fetch('/api/wallet/balance', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => {
        if (res.status === 401) {
          forceLogout()
        } else if (res.ok) {
          return res.json().then(data => {
            setState(s => ({ ...s, balance: data.balance ?? 0 }))
          })
        }
      })
      .catch(() => { /* ignore — server might be temporarily unreachable */ })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // --- authFetch: injects auth header, triggers forceLogout on 401 ---
  const authFetch = useCallback(
    async (url: string, options: RequestInit = {}): Promise<Response> => {
      const headers = new Headers(options.headers)
      if (state.token) headers.set('Authorization', `Bearer ${state.token}`)
      const res = await fetch(url, { ...options, headers })
      if (res.status === 401) forceLogout()
      return res
    },
    [state.token, forceLogout],
  )

  const login = useCallback(async (username: string, password: string) => {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error ?? 'Login failed')
    const newState = { token: data.token, userId: data.user_id, username, balance: 0 }
    persist(newState)
    try {
      const br = await fetch('/api/wallet/balance', {
        headers: { Authorization: `Bearer ${data.token}` },
      })
      const bd = await br.json()
      persist({ ...newState, balance: bd.balance ?? 0 })
    } catch { /* ignore */ }
  }, [])

  const register = useCallback(async (username: string, password: string) => {
    const res = await fetch('/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error ?? 'Registration failed')
    persist({ token: data.token, userId: data.user_id, username, balance: data.balance ?? 0 })
  }, [])

  const refreshBalance = useCallback(async () => {
    if (!state.token) return
    try {
      const res = await fetch('/api/wallet/balance', {
        headers: { Authorization: `Bearer ${state.token}` },
      })
      const data = await res.json()
      setState(s => ({ ...s, balance: data.balance ?? 0 }))
    } catch { /* ignore */ }
  }, [state.token])

  return (
    <AuthContext.Provider
      value={{
        ...state,
        login,
        register,
        logout,
        refreshBalance,
        sessionExpired,
        clearSessionExpired,
        authFetch,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
