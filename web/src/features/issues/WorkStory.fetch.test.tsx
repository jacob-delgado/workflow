import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { startWork, startWorkInWorktree } from './startWorkApi.ts'
import { WorkStory } from './WorkStory.tsx'

vi.mock('./startWorkApi.ts', () => ({
  startWork: vi.fn(() => Promise.resolve(makeBranch())),
  startWorkInWorktree: vi.fn(() =>
    Promise.resolve({ dir: '/home/ana/src/api-x', shown: '~/src/api-x', branch: 'feat/PROJ-999' }),
  ),
}))
const mockStartWork = vi.mocked(startWork)
const mockStartInWorktree = vi.mocked(startWorkInWorktree)

vi.mock('@/features/repositories/repositoriesApi.ts', () => ({ useSwitchTo: () => vi.fn() }))

// fetchFailed is the server's refusal of a start of work whose fetch failed.
const fetchFailed = {
  code: 'fetch_failed',
  detail: 'origin could not be fetched, so nothing was made for PROJ-999',
}

beforeEach(() => {
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
})

test('fetches origin before it starts work', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<WorkStory issueKey="PROJ-999" />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Start work' }))

  // Assert
  expect(mockStartWork).toHaveBeenLastCalledWith('PROJ-999', true)
})

test('offers to branch from what you have when the fetch fails, as the terminal does', async () => {
  // Arrange
  mockStartWork.mockRejectedValueOnce(fetchFailed)
  const user = userEvent.setup()
  render(<WorkStory issueKey="PROJ-999" />)
  await user.click(screen.getByRole('button', { name: 'Start work' }))

  // Act
  await user.click(await screen.findByRole('button', { name: 'Branch from what you have' }))

  // Assert
  expect(screen.queryByText(/could not be fetched/)).toBeNull()
  expect(mockStartWork).toHaveBeenLastCalledWith('PROJ-999', false)
})

test('says why the fetch failed beside the way out', async () => {
  // Arrange
  mockStartWork.mockRejectedValueOnce(fetchFailed)
  const user = userEvent.setup()
  render(<WorkStory issueKey="PROJ-999" />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Start work' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toContain('could not be fetched')
})

test('offers the same way out for a new worktree', async () => {
  // Arrange
  mockStartInWorktree.mockRejectedValueOnce(fetchFailed)
  const user = userEvent.setup()
  render(<WorkStory issueKey="PROJ-999" />)
  await user.click(screen.getByRole('button', { name: 'Start work in a new worktree' }))

  // Act
  await user.click(await screen.findByRole('button', { name: 'Branch from what you have' }))

  // Assert
  expect(mockStartInWorktree).toHaveBeenLastCalledWith('PROJ-999', false)
})

test('offers no way out for a refusal that is not the fetch', async () => {
  // Arrange
  mockStartWork.mockRejectedValueOnce({ code: 'conflict', detail: 'a branch already exists' })
  const user = userEvent.setup()
  render(<WorkStory issueKey="PROJ-999" />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Start work' }))

  // Assert
  await screen.findByRole('alert')
  expect(screen.queryByRole('button', { name: 'Branch from what you have' })).toBeNull()
})
