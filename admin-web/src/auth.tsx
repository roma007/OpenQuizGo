import { createContext, useContext } from 'react'
import type { AdminPrincipal } from './types'

interface AuthContextValue {
  principal: AdminPrincipal
  can: (permission: string) => boolean
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext)
  if (!value) throw new Error('AuthContext missing')
  return value
}
