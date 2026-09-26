import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { api, setUnauthorizedHandler, tokenStore } from '../api/client'
import type { LoginResult } from '../api/types'
import { AuthContext, useAuth, type Session } from './useAuth'

const SESSION_KEY = 'energyai.session'

function readSession(): Session | null {
  if (!tokenStore.get()) return null
  try {
    const raw = localStorage.getItem(SESSION_KEY)
    return raw ? (JSON.parse(raw) as Session) : null
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(readSession)
  const qc = useQueryClient()

  const logout = useCallback(() => {
    tokenStore.set(null)
    try {
      localStorage.removeItem(SESSION_KEY)
    } catch {
      /* ignore */
    }
    qc.clear()
    setSession(null)
  }, [qc])

  useEffect(() => {
    setUnauthorizedHandler(logout)
    return () => setUnauthorizedHandler(null)
  }, [logout])

  const login = useCallback(async (email: string, password: string) => {
    const res = await api<LoginResult>('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) })
    tokenStore.set(res.token)
    const s = { email: res.user.email, name: res.user.name }
    try {
      localStorage.setItem(SESSION_KEY, JSON.stringify(s))
    } catch {
      /* ignore */
    }
    setSession(s)
  }, [])

  const value = useMemo(() => ({ session, login, logout }), [session, login, logout])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const { session } = useAuth()
  const location = useLocation()
  if (!session) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <>{children}</>
}
