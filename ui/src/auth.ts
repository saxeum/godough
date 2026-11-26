// The two hand-written auth operations. The session endpoints are browser
// navigation/control, not generated API calls: no token, secret or client id is
// in the bundle.
import { ApiError } from './api/http'

export type Session = {
  email: string
  name: string
}

export async function getSession(): Promise<Session | null> {
  const res = await fetch('/api/auth/session', { headers: { Accept: 'application/json' } })
  if (res.status === 401) return null
  if (!res.ok) throw new ApiError(res.status)
  return (await res.json()) as Session
}

export async function signOut(): Promise<void> {
  const res = await fetch('/api/auth/signout', { method: 'POST', headers: { Accept: 'application/json' } })
  if (!res.ok) throw new ApiError(res.status)
}
