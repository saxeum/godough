// The API is same-origin: nginx routes /api to the api container locally, and
// the ingress does the same in production, so there is no base URL and no CORS.
export type Snippet = {
  ID: number
  Title: string
  Content: string
  Created: string
  Expires: string
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } })
  if (!res.ok) {
    throw new Error(res.status === 404 ? 'not found' : `request failed with ${res.status}`)
  }
  return (await res.json()) as T
}

export const listSnippets = () => getJSON<Snippet[]>('/api/snippets')

export const getSnippet = (id: string) => getJSON<Snippet>(`/api/snippet/view/${id}`)
