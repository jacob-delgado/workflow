import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { makeHealth, makeSnapshot } from '@/test/fixtures.ts'
import { client } from './generated/client.gen.ts'
import { useHealthStore } from './health.ts'
import { useSnapshotStore } from './snapshot.ts'
import './client.ts'

// The commit form beside the push reads the configuration; with none it offers
// the built-in types, and these tests never reach it.
vi.mock('@/features/settings/configApi.ts', () => ({ useConfig: () => ({ data: undefined }) }))

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
  // Empty, not the contract's absolute http://127.0.0.1:7000 — an absolute URL
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
