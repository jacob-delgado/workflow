import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { makeHealth, makeSnapshot } from '@/test/fixtures.ts'
import { client } from './generated/client.gen.ts'
import { getHealth } from './generated/sdk.gen.ts'
import { useHealthStore } from './health.ts'
import { useSessionStore } from './session.ts'
import { useSnapshotStore } from './snapshot.ts'
import './client.ts'

// fakeServer stands in for fetch, answering every request with the snapshot's
// branch (what a push returns), and records each request's method so a test can
// count what left the browser.
function fakeServer() {
  const methods: string[] = []
  const fetch = vi.fn((request: Request) => {
    methods.push(request.method)

    return Promise.resolve(Response.json(makeSnapshot().branch))
  })
  vi.stubGlobal('fetch', fetch)

  return methods
}

// pushTheBranch renders the branch panel over a branch with commits to push and
// confirms the push, as a user would.
async function pushTheBranch() {
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: /push branch/i }))
  await user.click(screen.getByRole('button', { name: /^push$/i }))
}

test('makes API requests relative to the page origin, not the spec server URL', () => {
  // Act
  const baseUrl = client.getConfig().baseUrl

  // Assert
  // Empty, not the contract's absolute http://127.0.0.1:13579 — an absolute URL
  // would bypass the dev proxy and hit a CORS wall.
  expect(baseUrl).toBe('')
})

test('clicking Push under dry run reports the hold without a request', async () => {
  // Arrange
  const methods = fakeServer()
  useHealthStore.setState({ health: makeHealth({ dry_run: true }) })

  // Act
  await pushTheBranch()

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toMatch(/held back by --dry-run/i)
  expect(methods).toEqual([])
})

test('clicking Push without dry run sends the push', async () => {
  // Arrange
  const methods = fakeServer()
  useHealthStore.setState({ health: makeHealth() })

  // Act
  await pushTheBranch()

  // Assert
  await vi.waitFor(() => {
    expect(methods).toEqual(['POST'])
  })
})

test('lets a read through under dry run', async () => {
  // Arrange
  const methods = fakeServer()
  useHealthStore.setState({ health: makeHealth({ dry_run: true }) })

  // Act
  await client.get({ url: '/api/branch' })

  // Assert
  expect(methods).toEqual(['GET'])
})

test('a write names the directory the page shows', async () => {
  // Arrange
  const headers: (string | null)[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      headers.push(request.headers.get('Workflow-Here'))

      return Promise.resolve(Response.json(makeSnapshot().branch))
    }),
  )
  useHealthStore.setState({ health: makeHealth({ dry_run: false }) })

  // Act
  await pushTheBranch()

  // Assert
  // The server refuses a write naming another directory than it works in,
  // so a page that has not noticed a switch cannot write to the new one. A
  // header carries bytes, so the path goes escaped.
  await vi.waitFor(() => {
    expect(headers).toEqual(['%2Fhome%2Fana%2Fsrc%2Fapi'])
  })
})

test('a directory named outside ASCII still names itself on a write', async () => {
  // Arrange
  const headers: (string | null)[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      headers.push(request.headers.get('Workflow-Here'))

      return Promise.resolve(Response.json(makeSnapshot().branch))
    }),
  )
  useHealthStore.setState({ health: makeHealth({ dry_run: false }) })
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot({ here: '/home/josé/日本' }) })
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /push branch/i }))
  await user.click(screen.getByRole('button', { name: /^push$/i }))

  // Assert
  await vi.waitFor(() => {
    expect(headers).toEqual([encodeURIComponent('/home/josé/日本')])
  })
})

// presentedAuthorization stands in for fetch, answering every request with the
// server's health, and records the Authorization each request presented.
function presentedAuthorization() {
  const presented: (string | null)[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      presented.push(request.headers.get('Authorization'))

      return Promise.resolve(Response.json(makeHealth()))
    }),
  )

  return presented
}

test("every request presents the page's session as a bearer token", async () => {
  // Arrange
  const presented = presentedAuthorization()
  useSessionStore.setState({ token: 'ABC234' })

  // Act
  await getHealth()

  // Assert
  expect(presented).toEqual(['Bearer ABC234'])
})

test('a page holding no session presents none', async () => {
  // Arrange
  const presented = presentedAuthorization()

  // Act
  await getHealth()

  // Assert
  expect(presented).toEqual([null])
})

test('an answer refusing the session marks it refused', async () => {
  // Arrange
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        Response.json(
          {
            type: 'about:blank',
            title: 'Unauthorized',
            status: 401,
            detail: 'no session',
            code: 'unauthorized',
          },
          { status: 401, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    ),
  )

  // Act
  await getHealth()

  // Assert
  expect(useSessionStore.getState().refused).toBe(true)
})
