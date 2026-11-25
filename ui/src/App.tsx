import { useEffect, useState } from 'react'
import { Link, Route, Routes, useNavigate, useParams } from 'react-router-dom'
import { ApiError, deleteSnippet, getSnippet, listSnippets, type Snippet } from './api'
import CreateSnippet from './CreateSnippet'

type State<T> = { status: 'loading' } | { status: 'failed'; error: Error } | { status: 'ok'; data: T }

// useFetched runs load() once per key change and keeps the three states a
// fetch can be in, so each page below only renders what it got.
function useFetched<T>(load: () => Promise<T>, key: unknown): State<T> {
  const [state, setState] = useState<State<T>>({ status: 'loading' })

  useEffect(() => {
    let current = true
    setState({ status: 'loading' })
    load().then(
      (data) => current && setState({ status: 'ok', data }),
      (error: Error) => current && setState({ status: 'failed', error }),
    )
    return () => {
      current = false
    }
    // load is recreated every render; key is the identity that matters.
  }, [key])

  return state
}

function Message({ children }: { children: React.ReactNode }) {
  return (
    <main>
      <p>{children}</p>
    </main>
  )
}

function SnippetList() {
  const state = useFetched(listSnippets, 'list')

  if (state.status === 'loading') return <Message>Loading…</Message>
  if (state.status === 'failed') return <Message>Could not load snippets: {state.error.message}</Message>
  if (state.data.length === 0) return <Message>No snippets yet.</Message>

  return (
    <main>
      <h2>Latest Snippets</h2>
      {state.data.map((s) => (
        <div className="snippet" key={s.ID}>
          <div className="metadata">
            <strong>
              <Link to={`/snippet/${s.ID}`}>{s.Title}</Link>
            </strong>
            <span>{new Date(s.Expires).toLocaleDateString()}</span>
          </div>
          <pre>{s.Content}</pre>
        </div>
      ))}
    </main>
  )
}

function SnippetView() {
  const { id = '' } = useParams()
  const state = useFetched(() => getSnippet(id), id)
  const navigate = useNavigate()
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')

  async function remove(s: Snippet) {
    if (!window.confirm(`Delete "${s.Title}"?`)) return
    setDeleting(true)
    setDeleteError('')
    try {
      await deleteSnippet(s.ID)
      navigate('/')
    } catch (err) {
      setDeleteError(err instanceof ApiError ? `Could not delete (${err.status}).` : 'Could not reach the server.')
      setDeleting(false)
    }
  }

  if (state.status === 'loading') return <Message>Loading…</Message>
  if (state.status === 'failed') {
    const missing = state.error instanceof ApiError && state.error.status === 404
    return missing ? <Message>That snippet does not exist.</Message> : <Message>Could not load the snippet: {state.error.message}</Message>
  }

  const s: Snippet = state.data
  return (
    <main>
      <h2>{s.Title}</h2>
      <div className="snippet">
        <div className="metadata">
          <strong>Created</strong>
          <time>{new Date(s.Created).toLocaleString()}</time>
          <span>Expires {new Date(s.Expires).toLocaleDateString()}</span>
        </div>
        <pre>{s.Content}</pre>
      </div>
      {deleteError && <p className="error">{deleteError}</p>}
      <button type="button" onClick={() => remove(s)} disabled={deleting}>
        {deleting ? 'Deleting…' : 'Delete snippet'}
      </button>
    </main>
  )
}

function App() {
  return (
    <>
      <header>
        <h1>
          <Link to="/">Snippetbox</Link>
        </h1>
      </header>
      <nav>
        <Link to="/">Home</Link>
        <Link to="/snippet/create">New snippet</Link>
      </nav>
      <Routes>
        <Route path="/" element={<SnippetList />} />
        <Route path="/snippet/create" element={<CreateSnippet />} />
        <Route path="/snippet/:id" element={<SnippetView />} />
        <Route path="*" element={<Message>Page not found.</Message>} />
      </Routes>
      <footer>
        Powered by <a href="https://golang.org/">Go</a>
      </footer>
    </>
  )
}

export default App
