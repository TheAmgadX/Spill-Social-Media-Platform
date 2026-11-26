import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, setAccessToken, setSessionExpiredHandler } from './api'
import { AuthContext } from './auth-context'
import type { PrivateUser } from '../types'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<PrivateUser | null>(null)
  const [loading, setLoading] = useState(true)
  const [notice, setNotice] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    setSessionExpiredHandler(() => {
      if (active) {
        setUser(null)
        setNotice('Your session expired. Sign in again to continue.')
      }
    })

    void api.refresh()
      .then(async (token) => {
        if (!active || !token) return
        setAccessToken(token)
        const currentUser = await api.getMe()
        if (active) setUser(currentUser)
      })
      .catch((error: unknown) => {
        if (active) {
          setNotice(error instanceof Error ? error.message : 'Could not restore your session.')
        }
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
      setSessionExpiredHandler(null)
    }
  }, [])

  const establishSession = useCallback(async (token: string) => {
    setAccessToken(token)
    const currentUser = await api.getMe()
    setUser(currentUser)
    setNotice(null)
  }, [])

  const signOut = useCallback(async () => {
    let logoutError: unknown
    try {
      if (user) await api.logout()
    } catch (error) {
      logoutError = error
    } finally {
      setAccessToken(null)
      setUser(null)
    }
    if (logoutError) {
      const message = logoutError instanceof Error ? logoutError.message : 'Server logout failed.'
      setNotice(`Signed out locally, but the server logout failed: ${message}`)
      throw logoutError
    }
  }, [user])

  const dismissNotice = useCallback(() => setNotice(null), [])
  const value = useMemo(
    () => ({ user, loading, notice, establishSession, signOut, dismissNotice }),
    [user, loading, notice, establishSession, signOut, dismissNotice],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
