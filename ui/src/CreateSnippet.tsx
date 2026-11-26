import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError, createSnippet } from './api'

const EXPIRIES = [
  { days: 1, label: '1 day' },
  { days: 7, label: '1 week' },
  { days: 365, label: '1 year' },
]

export default function CreateSnippet() {
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [days, setDays] = useState(7)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [failed, setFailed] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    setBusy(true)
    setFieldErrors({})
    setFailed('')

    try {
      const created = await createSnippet({ title, content, expiresInDays: days })
      navigate(`/snippet/${created.id}`)
    } catch (err) {
      if (err instanceof ApiError && Object.keys(err.fieldErrors).length > 0) {
        setFieldErrors(err.fieldErrors)
      } else if (err instanceof ApiError) {
        setFailed(`Could not save the snippet (${err.status}).`)
      } else {
        setFailed('Could not reach the server.')
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <main>
      <h2>Create a snippet</h2>
      {failed && <p className="error">{failed}</p>}
      <form onSubmit={submit}>
        <label htmlFor="title">Title</label>
        <input id="title" name="title" value={title} onChange={(e) => setTitle(e.target.value)} />
        {fieldErrors.title && <p className="error">{fieldErrors.title}</p>}

        <label htmlFor="content">Content</label>
        <textarea id="content" name="content" rows={8} value={content} onChange={(e) => setContent(e.target.value)} />
        {fieldErrors.content && <p className="error">{fieldErrors.content}</p>}

        <label htmlFor="expires">Expires in</label>
        <select id="expires" name="expires" value={days} onChange={(e) => setDays(Number(e.target.value))}>
          {EXPIRIES.map((e) => (
            <option key={e.days} value={e.days}>
              {e.label}
            </option>
          ))}
        </select>
        {fieldErrors.expiresInDays && <p className="error">{fieldErrors.expiresInDays}</p>}

        <button type="submit" disabled={busy}>
          {busy ? 'Saving…' : 'Save snippet'}
        </button>
      </form>
    </main>
  )
}
