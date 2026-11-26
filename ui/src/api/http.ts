// The one hand-written piece of the API client: fetch, and what a failure means.
// orval's generated functions call this as their mutator, so every endpoint
// throws the same error type and a 422 still arrives with its per-field messages.

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

export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...init?.headers,
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

  // 204 has no body by definition; deleteSnippet is the only caller today.
  if (res.status === 204) {
    return undefined as T
  }

  return (await res.json()) as T
}
