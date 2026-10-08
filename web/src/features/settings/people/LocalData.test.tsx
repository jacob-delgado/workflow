import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { LocalData as Listing } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeHealth } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { LocalData } from './LocalData.tsx'

const dir = '/home/ana/.local/state/workflow'

const cache = {
  name: 'workflow.db',
  kind: 'cache',
  bytes: 94_208,
  size: '92.0 KiB',
  holds: [
    { what: 'scopes', count: 3 },
    { what: 'cached issues', count: 41 },
  ],
} satisfies Listing['files'][number]

const kept = {
  name: 'kept.db',
  kind: 'kept',
  bytes: 512,
  size: 'half a KiB',
  holds: [{ what: 'repository groups', count: 2 }],
} satisfies Listing['files'][number]

// consequences are what the server says each removal takes with it, which the
// page warns with as they are.
const consequences = {
  cache: 'The cache comes back as you work.',
  all: 'Who is who goes with it, and will be asked again.',
} satisfies Listing['consequences']

// storeHolding answers the listing with files until a removal takes what its
// scope reaches, recording each request.
function storeHolding(files: Listing['files']): Request[] {
  let left = files

  return fakeApi({
    '/api/local-data': (at: URL, asked: Request) => {
      if (asked.method === 'DELETE') {
        left = at.searchParams.get('scope') === 'all' ? [] : left.filter((f) => f.kind === 'kept')
      }

      return { dir, files: left, consequences }
    },
  })
}

// refusing answers the listing with both files and refuses a removal with a 409.
function refusing(): void {
  fakeApi({
    '/api/local-data': (_: URL, asked: Request) =>
      asked.method === 'DELETE'
        ? Response.json(
            { status: 409, code: 'conflict', title: 'Conflict', type: '', detail: 'held open' },
            { status: 409 },
          )
        : { dir, files: [cache, kept], consequences },
  })
}

test('lists the directory and each file with its kind, size and what it holds', async () => {
  // Arrange
  storeHolding([cache, kept])

  // Act
  renderWithClient(<LocalData />)

  // Assert
  const table = await screen.findByRole('table', { name: 'Local data files' })
  expect(screen.getByText(dir)).toBeTruthy()
  const rows = within(table).getAllByRole('row')
  expect(rows.map((row) => row.textContent)).toEqual([
    'FileKindSizeHolds',
    'workflow.dbCache92.0 KiBscopes: 3, cached issues: 41',
    'kept.dbKepthalf a KiBrepository groups: 2',
  ])
})

test('says when there is no local data', async () => {
  // Arrange
  storeHolding([])

  // Act
  renderWithClient(<LocalData />)

  // Assert
  expect(await screen.findByText(/no local data/i)).toBeTruthy()
  expect(screen.queryByRole('button', { name: /remove/i })).toBeNull()
})

test('Remove cache… asks first, in a group that takes focus, and Cancel sends nothing', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = storeHolding([cache, kept])
  renderWithClient(<LocalData />)
  await user.click(await screen.findByRole('button', { name: 'Remove cache…' }))
  const question = screen.getByRole('group', { name: /remove workflow\.db\?/i })
  expect(question.contains(document.activeElement)).toBe(true)

  // Act
  await user.click(within(question).getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(screen.queryByRole('group')).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Remove cache…' }))
  expect(requests.map((request) => request.method)).toEqual(['GET'])
})

test('Remove everything… warns as the server says, removes and reads again', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = storeHolding([cache, kept])
  renderWithClient(<LocalData />)
  await user.click(await screen.findByRole('button', { name: 'Remove everything…' }))
  const question = screen.getByRole('group', { name: /remove workflow\.db and kept\.db\?/i })
  expect(question.textContent).toContain(consequences.all)

  // Act
  await user.click(within(question).getByRole('button', { name: 'Remove' }))

  // Assert
  expect(await screen.findByRole('status')).toHaveProperty(
    'textContent',
    'Removed workflow.db and kept.db.',
  )
  await waitFor(() => {
    expect(requests.map((request) => request.method)).toEqual(['GET', 'DELETE', 'GET'])
  })
  expect(new URL(requests[1]?.url ?? '').searchParams.get('scope')).toBe('all')
  expect(await screen.findByText(/no local data/i)).toBeTruthy()
})

test('removing the cache leaves the kept file listed', async () => {
  // Arrange
  const user = userEvent.setup()
  storeHolding([cache, kept])
  renderWithClient(<LocalData />)
  await user.click(await screen.findByRole('button', { name: 'Remove cache…' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Remove' }))

  // Assert
  await waitFor(() => {
    expect(screen.getByRole('status').textContent).toBe('Removed workflow.db.')
  })
  const table = screen.getByRole('table', { name: 'Local data files' })
  expect(within(table).queryByText('workflow.db')).toBeNull()
  expect(within(table).getByText('kept.db')).toBeTruthy()
})

test('a removal that could not remove a file says why', async () => {
  // Arrange
  const user = userEvent.setup()
  refusing()
  renderWithClient(<LocalData />)
  await user.click(await screen.findByRole('button', { name: 'Remove everything…' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Remove' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('held open')
  expect(screen.getByRole('table', { name: 'Local data files' })).toBeTruthy()
})

test('under --dry-run the removals are held back, and the page says so', async () => {
  // Arrange
  useHealthStore.setState({ health: makeHealth({ dry_run: true }) })
  storeHolding([cache, kept])

  // Act
  renderWithClient(<LocalData />)

  // Assert
  await screen.findByRole('table', { name: 'Local data files' })
  expect(screen.queryByRole('button', { name: /remove/i })).toBeNull()
  expect(screen.getByText(/removing is held back/i)).toBeTruthy()
})

test('a listing that could not be read says so and offers to read again', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({})
  renderWithClient(<LocalData />)
  const again = await screen.findByRole('button', { name: 'Try again' })
  storeHolding([cache])

  // Act
  await user.click(again)

  // Assert
  expect(await screen.findByRole('table', { name: 'Local data files' })).toBeTruthy()
})

test('the mockup lists its store without a server', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderWithClient(<LocalData />)

  // Assert
  const table = await screen.findByRole('table', { name: 'Local data files' })
  expect(within(table).getByText('kept.db')).toBeTruthy()
  expect(globalThis.fetch).not.toHaveBeenCalled()
})
