import { createContext, useContext, useState, useEffect, useCallback } from 'react'

export type Role = 'guest' | 'member' | 'admin'

interface UserInfo {
  username: string
  role: Role
}

interface AuthContextValue {
  token: string | null
  user: UserInfo | null
  login: (token: string) => void
  logout: () => void
  isGuest: boolean
  canWrite: boolean  // member or admin
  isAdmin: boolean
  isLoading: boolean
}

const AuthContext = createContext<AuthContextValue | null>(null)

function decodeRole(token: string): Role {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    const r = payload.role
    if (r === 'admin' || r === 'member') return r
  } catch {
    // malformed token
  }
  return 'member'
}

function decodeUsername(token: string): string {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return payload.sub ?? ''
  } catch {
    return ''
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('jwt_token'))
  const [isLoading] = useState(false)

  const user: UserInfo | null = token
    ? { username: decodeUsername(token), role: decodeRole(token) }
    : null

  const login = useCallback((newToken: string) => {
    localStorage.setItem('jwt_token', newToken)
    setToken(newToken)
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem('jwt_token')
    setToken(null)
  }, [])

  // Keep token state in sync with localStorage (other tabs)
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === 'jwt_token') {
        setToken(e.newValue)
      }
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  const isGuest = !token
  const canWrite = user?.role === 'member' || user?.role === 'admin'
  const isAdmin = user?.role === 'admin'

  return (
    <AuthContext.Provider value={{ token, user, login, logout, isGuest, canWrite, isAdmin, isLoading }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
