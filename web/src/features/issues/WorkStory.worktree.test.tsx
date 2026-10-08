import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot, makeTaskBranch } from '@/test/fixtures.ts'
import { startWorkInWorktree } from './startWorkApi.ts'
import { WorkStory } from './WorkStory.tsx'

// Work can start in a new worktree beside the repository, which the story
// then offers to switch to, and a branch another worktree holds is switched
// to there rather than checked out.

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
const mockStartInWorktree = vi.mocked(startWorkInWorktree)

const mockSwitchTo = vi.fn((dir: string) =>
  Promise.resolve({ here: { shown: dir.replace('/home/ana', '~') } }),
)
vi.mock('@/features/repositories/repositoriesApi.ts', () => ({ useSwitchTo: () => mockSwitchTo }))

test('offers to start work in a new worktree beside the repository', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })

  // Act
  render(<WorkStory issueKey="PROJ-999" />)

  // Assert
  expect(screen.getByRole('button', { name: 'Start work in a new worktree' })).toBeTruthy()
})

test('work started in a new worktree offers to switch to it, and switches', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-999" />)

  // Act: start work in a worktree
  await user.click(screen.getByRole('button', { name: 'Start work in a new worktree' }))

  // Assert: it says where, and nothing is switched yet
  const offer = await screen.findByRole('region', {
    name: 'Started PROJ-999 in ~/src/api-feat-PROJ-999',
  })
  expect(mockStartInWorktree).toHaveBeenCalledWith('PROJ-999', true)
  expect(mockSwitchTo).not.toHaveBeenCalled()

  // Act: switch to it
  await user.click(within(offer).getByRole('button', { name: 'Switch to it' }))

  // Assert: it asks first, and nothing is switched yet
  const confirm = screen.getByRole('region', { name: 'Switch to ~/src/api-feat-PROJ-999?' })
  expect(mockSwitchTo).not.toHaveBeenCalled()

  // Act: confirm the switch
  await user.click(within(confirm).getByRole('button', { name: 'Switch' }))

  // Assert: switched
  expect(mockSwitchTo).toHaveBeenCalledWith('/home/ana/src/api-feat-PROJ-999')
  expect(await screen.findByText('Switched to ~/src/api-feat-PROJ-999.')).toBeTruthy()
})

test('the offer to switch stays once the snapshot shows the new branch', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-999" />)
  await user.click(screen.getByRole('button', { name: 'Start work in a new worktree' }))
  await screen.findByRole('button', { name: 'Switch to it' })

  // Act
  act(() => {
    useSnapshotStore.setState({
      status: 'live',
      snapshot: makeSnapshot({
        branches: [
          makeTaskBranch({
            name: 'feat/PROJ-999',
            issue_key: 'PROJ-999',
            current: false,
            worktree: '/home/ana/src/api-feat-PROJ-999',
            worktree_shown: '~/src/api-feat-PROJ-999',
          }),
        ],
      }),
    })
  })

  // Assert
  expect(screen.getByRole('button', { name: 'Switch to it' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Start work' })).toBeNull()
  expect(screen.queryByRole('button', { name: /switch to its worktree/i })).toBeNull()
})

test('a branch another worktree has checked out is switched to there, not checked out', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [
        makeTaskBranch({ name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true }),
        makeTaskBranch({
          name: 'feat/PROJ-2-metrics',
          issue_key: 'PROJ-2',
          current: false,
          worktree: '/home/ana/src/api-feat-PROJ-2-metrics',
          worktree_shown: '~/src/api-feat-PROJ-2-metrics',
        }),
      ],
    }),
  })
  render(<WorkStory issueKey="PROJ-2" />)

  // Act: switch to its worktree
  await user.click(
    screen.getByRole('button', { name: 'Switch to its worktree, ~/src/api-feat-PROJ-2-metrics' }),
  )

  // Assert: it asks first, and nothing is switched yet
  const confirm = screen.getByRole('region', { name: 'Switch to ~/src/api-feat-PROJ-2-metrics?' })
  expect(screen.queryByRole('button', { name: 'Switch branch' })).toBeNull()
  expect(mockSwitchTo).not.toHaveBeenCalled()

  // Act: confirm the switch
  await user.click(within(confirm).getByRole('button', { name: 'Switch' }))

  // Assert: switched there, nothing checked out
  expect(mockSwitchTo).toHaveBeenCalledWith('/home/ana/src/api-feat-PROJ-2-metrics')
  expect(await screen.findByText('Switched to ~/src/api-feat-PROJ-2-metrics.')).toBeTruthy()
})

test('the offer goes once the switch has made the branch the one checked out', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-999" />)
  await user.click(screen.getByRole('button', { name: 'Start work in a new worktree' }))
  await user.click(await screen.findByRole('button', { name: 'Switch to it' }))
  await user.click(screen.getByRole('button', { name: 'Switch' }))

  // Act
  act(() => {
    useSnapshotStore.setState({
      status: 'live',
      snapshot: makeSnapshot({
        here: '/home/ana/src/api-feat-PROJ-999',
        branches: [makeTaskBranch({ name: 'feat/PROJ-999', issue_key: 'PROJ-999', current: true })],
      }),
    })
  })

  // Assert
  expect(screen.queryByRole('button', { name: 'Switch to it' })).toBeNull()
})

test('a refused switch from the offer is announced', async () => {
  // Arrange
  mockSwitchTo.mockRejectedValueOnce({ code: 'conflict', detail: 'a write is being made' })
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-999" />)
  await user.click(screen.getByRole('button', { name: 'Start work in a new worktree' }))
  await user.click(await screen.findByRole('button', { name: 'Switch to it' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toMatch(/a write is being made/)
})

test('a branch held by a worktree that is gone says how to free it', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [
        makeTaskBranch({ name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true }),
        makeTaskBranch({
          name: 'feat/PROJ-2-metrics',
          issue_key: 'PROJ-2',
          current: false,
          worktree: '/home/ana/src/api-gone',
          worktree_shown: '~/src/api-gone',
          worktree_missing: true,
        }),
      ],
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-2" />)

  // Assert
  expect(screen.getByText(/git worktree prune/)).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Switch branch' })).toBeNull()
  expect(screen.queryByRole('button', { name: /switch to its worktree/i })).toBeNull()
})

test('a worktree that could not be made says why, with focus still on its button', async () => {
  // Arrange
  mockStartInWorktree.mockRejectedValueOnce({
    code: 'conflict',
    detail: 'a branch for this issue already exists',
  })
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-999" />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Start work in a new worktree' }))

  // Assert
  expect(await screen.findByText(/already exists/i)).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Switch to it' })).toBeNull()
  expect(document.activeElement).toBe(
    screen.getByRole('button', { name: 'Start work in a new worktree' }),
  )
})
