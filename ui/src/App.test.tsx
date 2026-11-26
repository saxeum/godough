import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import App from './App'
import CreateSnippet from './CreateSnippet'
import { ApiError, createSnippet, getSnippet, listSnippets } from './api'
import { getSession, signOut } from './auth'

vi.mock('./api', async () => {
  const actual = await vi.importActual<typeof import('./api')>('./api')
  return {
    ...actual,
    createSnippet: vi.fn(),
    deleteSnippet: vi.fn(),
    getSnippet: vi.fn(),
    listSnippets: vi.fn(),
  }
})

vi.mock('./auth', () => ({
  getSession: vi.fn(),
  signOut: vi.fn(),
}))

const mockedList = vi.mocked(listSnippets)
const mockedGet = vi.mocked(getSnippet)
const mockedCreate = vi.mocked(createSnippet)
const mockedSession = vi.mocked(getSession)
const mockedSignOut = vi.mocked(signOut)

const snippet = {
  id: 7,
  title: 'A rendered title',
  content: 'A rendered body',
  created: '2026-01-01T00:00:00Z',
  expires: '2026-02-01T00:00:00Z',
}

beforeEach(() => {
  vi.resetAllMocks()
  mockedSession.mockResolvedValue({ email: 'person@example.com', name: 'A Person' })
})

describe('App', () => {
  it('renders the list the API returns', async () => {
    mockedList.mockResolvedValue([snippet])

    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )

    expect(await screen.findByRole('link', { name: snippet.title })).toBeInTheDocument()
    expect(screen.getByText(snippet.content)).toBeInTheDocument()
    expect(screen.getByText('A Person')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
  })

  it('renders the signed-out state without loading the list', async () => {
    mockedSession.mockResolvedValue(null)

    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )

    const link = await screen.findByRole('link', { name: 'Continue with Google' })
    expect(link).toHaveAttribute('href', '/api/auth/google')
    expect(mockedList).not.toHaveBeenCalled()
  })

  it('signs out and returns to the signed-out state', async () => {
    const user = userEvent.setup()
    mockedSession
      .mockResolvedValueOnce({ email: 'person@example.com', name: 'A Person' })
      .mockResolvedValue(null)
    mockedSignOut.mockResolvedValue(undefined)
    mockedList.mockResolvedValue([])

    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )

    await user.click(await screen.findByRole('button', { name: 'Sign out' }))
    expect(mockedSignOut).toHaveBeenCalledOnce()
    expect(await screen.findByRole('link', { name: 'Continue with Google' })).toBeInTheDocument()
  })

  it('renders the signed-out state on a 401', async () => {
    mockedList.mockRejectedValue(new ApiError(401))

    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )

    expect(await screen.findByText('Sign in required.')).toBeInTheDocument()
  })

  it('renders the not-found state on a 404', async () => {
    mockedGet.mockRejectedValue(new ApiError(404))

    render(
      <MemoryRouter initialEntries={['/snippet/999']}>
        <App />
      </MemoryRouter>,
    )

    expect(await screen.findByText('That snippet does not exist.')).toBeInTheDocument()
  })
})

describe('CreateSnippet', () => {
  it('puts 422 field errors under the matching inputs', async () => {
    const user = userEvent.setup()
    mockedCreate.mockRejectedValue(
      new ApiError(422, {
        title: 'title is required',
        content: 'content is required',
      }),
    )

    render(
      <MemoryRouter>
        <CreateSnippet />
      </MemoryRouter>,
    )

    await user.click(screen.getByRole('button', { name: 'Save snippet' }))

    expect(await screen.findByText('title is required')).toBeInTheDocument()
    expect(screen.getByText('content is required')).toBeInTheDocument()
    expect(screen.getByLabelText('Title').nextElementSibling).toHaveTextContent('title is required')
    expect(screen.getByLabelText('Content').nextElementSibling).toHaveTextContent('content is required')
  })
})
