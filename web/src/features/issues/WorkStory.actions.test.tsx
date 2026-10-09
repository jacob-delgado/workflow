import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot, makeTaskBranch } from '@/test/fixtures.ts'
import { checkoutBranch } from './checkoutApi.ts'
import { startWork } from './startWorkApi.ts'
import { WorkStory } from './WorkStory.tsx'

// The writes that move an issue along from its work story: switching to the
// branch of an issue in flight elsewhere, and starting work on one not
// started, each offered only where it applies and each saying why when it is
// refused.

vi.mock('./checkoutApi.ts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./checkoutApi.ts')>()),
  checkoutBranch: vi.fn(() => Promise.resolve(makeBranch())),
}))
const mockCheckout = vi.mocked(checkoutBranch)

vi.mock('./startWorkApi.ts', () => ({
  startWork: vi.fn(() => Promise.resolve(makeBranch())),
  startWorkInWorktree: vi.fn(() =>
    Promise.resolve({
      dir: '/home/ana/src/api-feat-PROJ-999',
      shown: '~/src/api-feat-PROJ-999',
      branch: 'feat/PROJ-999',
    }),
  ),
}))
const mockStartWork = vi.mocked(startWork)

const mockSwitchTo = vi.fn((dir: string) =>
  Promise.resolve({ here: { shown: dir.replace('/home/ana', '~') } }),
)
vi.mock('@/features/repositories/repositoriesApi.ts', () => ({ useSwitchTo: () => mockSwitchTo }))

// offHead is a snapshot with PROJ-2 in flight on a branch that is not on HEAD.
function offHead() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [
        makeTaskBranch({ name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true }),
        makeTaskBranch({ name: 'feat/PROJ-2-metrics', issue_key: 'PROJ-2', current: false }),
      ],
    }),
  })
}

test('offers to switch to an in-flight branch that is not on HEAD', () => {
  // Arrange
  offHead()

  // Act
  render(<WorkStory issueKey="PROJ-2" />)

  // Assert
  expect(screen.getByRole('button', { name: 'Switch branch' })).toBeTruthy()
})

test('does not offer to switch to the branch already on HEAD', () => {
  // Arrange
  offHead()

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Switch branch' })).toBeNull()
})

test('checks out the branch when its button is clicked', async () => {
  // Arrange
  mockCheckout.mockResolvedValueOnce(makeBranch())
  const user = userEvent.setup()
  offHead()
  render(<WorkStory issueKey="PROJ-2" />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch' }))

  // Assert
  expect(mockCheckout).toHaveBeenCalledWith('feat/PROJ-2-metrics')
})

test('shows the reason when a checkout is refused, with focus still on Switch branch', async () => {
  // Arrange
  mockCheckout.mockRejectedValueOnce({
    code: 'conflict',
    detail: 'uncommitted changes — commit or stash first',
  })
  const user = userEvent.setup()
  offHead()
  render(<WorkStory issueKey="PROJ-2" />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch' }))

  // Assert
  expect(await screen.findByText(/uncommitted changes/i)).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Switch branch' }))
})

test('offers to start work on a not-started issue, in the one verb for it', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })

  // Act
  render(<WorkStory issueKey="PROJ-999" />)

  // Assert
  expect(screen.getByRole('button', { name: 'Start work' })).toBeTruthy()
  expect(screen.getByText(/appear here once you start work on it\./i)).toBeTruthy()
})

test('does not offer to start work on an issue already in flight', () => {
  // Arrange
  offHead()

  // Act
  render(<WorkStory issueKey="PROJ-2" />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Start work' })).toBeNull()
})

test('starts work when its button is clicked', async () => {
  // Arrange
  mockStartWork.mockResolvedValueOnce(makeBranch())
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-999" />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Start work' }))

  // Assert
  expect(mockStartWork).toHaveBeenCalledWith('PROJ-999', true)
})

test('shows the reason when starting work is refused, with focus still on Start work', async () => {
  // Arrange
  mockStartWork.mockRejectedValueOnce({
    code: 'conflict',
    detail: 'a branch for this issue already exists',
  })
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-999" />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Start work' }))

  // Assert
  expect(await screen.findByText(/already exists/i)).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Start work' }))
})
