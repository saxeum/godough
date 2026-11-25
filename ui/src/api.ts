// The API is same-origin: nginx routes /api to the api container locally, and
// the ingress does the same in production, so there is no base URL and no CORS.
export type Snippet = {
  ID: number
  Title: string
  Content: string
  Created: string
  Expires: string
}

export type CreateInput = {
  Title: string
  Content: string
  ExpiresInDays: number
}

// ApiError carries what a form needs: the status, and the per-field messages
// the API returns on 422.
export class ApiError extends Error {
  readonly status: number
  readonly fieldErrors: Record<string, string>

  constructor(status: number, fieldErrors: Record<string, string> = {}) {
    super(`request failed with ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.fieldErrors = fieldErrors
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
    },
  })

  if (!res.ok) {
    let fieldErrors: Record<string, string> = {}
    try {
      const body = (await res.json()) as { errors?: Record<string, string> }
      fieldErrors = body.errors ?? {}
    } catch {
      // Not JSON: leave fieldErrors empty, the status is still useful.
    }
    throw new ApiError(res.status, fieldErrors)
  }

  // 204 has no body by definition; DELETE is the only caller today.
  if (res.status === 204) {
    return undefined as T
  }

  return (await res.json()) as T
}

export const listSnippets = () => request<Snippet[]>('/api/snippets')

export const getSnippet = (id: string) => request<Snippet>(`/api/snippet/view/${id}`)

export const createSnippet = (input: CreateInput) =>
  request<Snippet>('/api/snippet/create', { method: 'POST', body: JSON.stringify(input) })

export const deleteSnippet = (id: number) => request<void>(`/api/snippet/view/${id}`, { method: 'DELETE' })
