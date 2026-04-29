import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { apiClient, clearAccessToken, getAccessToken, setAccessToken, type PermissionCode, type UserProfile } from '../lib/api'

type AuthContextValue = {
  token: string | null
  user: UserProfile | null
  permissions: PermissionCode[]
  isAuthenticated: boolean
  isReady: boolean
  signIn: (token: string, user?: UserProfile | null) => void
  signOut: () => void
  hasPermission: (permission: PermissionCode | PermissionCode[]) => boolean
  hasAnyPermission: (permissions: PermissionCode[]) => boolean
}

const AUTH_USER_KEY = 'game-admin.auth.user'

const AuthContext = createContext<AuthContextValue | null>(null)

function readStoredUser() {
  const raw = localStorage.getItem(AUTH_USER_KEY)
  if (!raw) {
    return null
  }

  try {
    return JSON.parse(raw) as UserProfile
  } catch {
    localStorage.removeItem(AUTH_USER_KEY)
    return null
  }
}

function persistUser(user: UserProfile | null | undefined) {
  if (!user) {
    localStorage.removeItem(AUTH_USER_KEY)
    return
  }

  localStorage.setItem(AUTH_USER_KEY, JSON.stringify(user))
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(null)
  const [user, setUser] = useState<UserProfile | null>(null)
  const [isReady, setIsReady] = useState(false)

  useEffect(() => {
    const storedToken = getAccessToken()
    const storedUser = readStoredUser()
    setToken(storedToken)
    setUser(storedUser)
    if (!storedToken) {
      setIsReady(true)
      return
    }
    let cancelled = false
    apiClient.getCurrentUser()
      .then((nextUser) => {
        if (cancelled) {
          return
        }
        persistUser(nextUser)
        setUser(nextUser)
      })
      .catch(() => {
        if (cancelled) {
          return
        }
        clearAccessToken()
        persistUser(null)
        setToken(null)
        setUser(null)
      })
      .finally(() => {
        if (!cancelled) {
          setIsReady(true)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  const signIn = useCallback((nextToken: string, nextUser?: UserProfile | null) => {
    setAccessToken(nextToken)
    persistUser(nextUser)
    setToken(nextToken)
    setUser(nextUser ?? null)
  }, [])

  const signOut = useCallback(() => {
    clearAccessToken()
    persistUser(null)
    setToken(null)
    setUser(null)
  }, [])

  const permissions = user?.permissions ?? []

  const hasAnyPermission = useCallback((requiredPermissions: PermissionCode[]) => {
    if (requiredPermissions.length === 0) {
      return true
    }

    return requiredPermissions.some((permission) => permissions.includes(permission))
  }, [permissions])

  const hasPermission = useCallback((permission: PermissionCode | PermissionCode[]) => {
    if (Array.isArray(permission)) {
      return permission.every((item) => permissions.includes(item))
    }

    return permissions.includes(permission)
  }, [permissions])

  const value = useMemo<AuthContextValue>(() => ({
    token,
    user,
    permissions,
    isAuthenticated: Boolean(token),
    isReady,
    signIn,
    signOut,
    hasPermission,
    hasAnyPermission,
  }), [hasAnyPermission, hasPermission, isReady, permissions, signIn, signOut, token, user])

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }

  return context
}
