import { createContext } from 'react'
import type { PrivateUser } from '../types'

export interface AuthContextValue {
  user: PrivateUser | null
  loading: boolean
  notice: string | null
  establishSession: (token: string) => Promise<void>
  signOut: () => Promise<void>
  dismissNotice: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)
