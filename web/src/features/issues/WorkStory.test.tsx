import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useHealthStore } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { gitLabWords, makeBranch, makeHealth, makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { checkoutBranch } from './checkoutApi.ts'
import { startWork, startWorkInWorktree } from './startWorkApi.ts'
import { WorkStory } from './WorkStory.tsx'

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
const mockStartInWorktree = vi.mocked(startWorkInWorktree)

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
        { name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true },
        { name: 'feat/PROJ-2-metrics', issue_key: 'PROJ-2', current: false },
      ],
    }),
  })
}

// makeSnapshot's branch is fix/PROJ-1; a branches entry marks PROJ-1 as the one
// checked out, so PROJ-1 owns the current work.
const onHead = [{ name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true }]

test('lays out the in-flight stages for the issue that owns the branch', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      changes: {
        changes: [
          { path: 'a.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
        ],
      },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText('Branch')).toBeTruthy()
  expect(screen.getByText('Pull request')).toBeTruthy()
  expect(screen.getByText(/1 file\(s\) to commit/)).toBeTruthy()
})

test("draws each stage as the mark of how far it has come, beside the stage's state", () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      changes: {
        changes: [
          { path: 'a.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
        ],
      },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  const stages = screen.getAllByRole('listitem')
  expect(stages.map(markShape)).toEqual([
    drawnMark('done'),
    drawnMark('in-flight'),
    drawnMark('not-started'),
    drawnMark('not-started'),
  ])
  expect(stages.map((stage) => within(stage).getByRole('button').textContent)).toEqual([
    expect.stringContaining('done'),
    expect.stringContaining('active'),
    expect.stringContaining('upcoming'),
    expect.stringContaining('upcoming'),
  ])
})

test('reads a committed clean tree and a green pull request', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      branch: {
        name: 'fix/PROJ-1',
        issue_link: '',
        detached: false,
        head: 'h1h2h3h',
        upstream: 'origin/fix/PROJ-1',
        push_remote: 'origin',
        ahead: 1,
        behind: 0,
        base: 'origin/main',
        commits: [{ hash: 'h1h2h3h4', subject: 'do the work' }],
      },
      changes: { changes: [] },
      review: {
        found: true,
        pull: {
          number: 128,
          url: 'https://x/128',
          title: 'the change',
          state: 'open',
          draft: false,
          approvals: 1,
          changes_requested: false,
          mergeable: 'clean',
        },
        ci: { state: 'passed', total: 1, done: 1, failed: 0, checks: [] },
      },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText(/working tree clean/i)).toBeTruthy()
  expect(screen.getByText('#128')).toBeTruthy()
  expect(screen.getByText('CI passed')).toBeTruthy()
  expect(screen.getByText('1 ahead')).toBeTruthy()
})

test('reads a fresh branch as nothing-committed and a pull request with no CI', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      review: {
        found: true,
        pull: {
          number: 42,
          url: 'https://x/42',
          title: 'the change',
          state: 'open',
          draft: false,
          approvals: 0,
          changes_requested: false,
          mergeable: 'unknown',
        },
      },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText(/nothing committed yet/i)).toBeTruthy()
  expect(screen.getByText('#42')).toBeTruthy()
})

test('shows a not-started story for an issue that does not own the branch', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })

  // Act
  render(<WorkStory issueKey="PROJ-999" />)

  // Assert
  expect(screen.getByText(/not in progress/i)).toBeTruthy()
  expect(screen.getByText(/no branch for this issue yet/i)).toBeTruthy()
})

test('shows an in-progress-elsewhere story for an issue on a branch not checked out', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [
        { name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true },
        { name: 'feat/PROJ-2-metrics', issue_key: 'PROJ-2', current: false },
      ],
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-2" />)

  // Assert
  expect(screen.getByText(/in progress on feat\/PROJ-2-metrics/i)).toBeTruthy()
  // Both the changes and pull-request stages defer to the checked-out branch.
  expect(screen.getAllByText(/shown for the checked-out branch/i)).toHaveLength(2)
})

test('jumps to a stage section when it is clicked', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-1" />)

  // Act
  await user.click(screen.getByRole('button', { name: /pull request/i }))

  // Assert
  expect(useUiStore.getState().section).toBe('review')
})

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

test('shows the reason when a checkout is refused', async () => {
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
  expect(mockStartWork).toHaveBeenCalledWith('PROJ-999')
})

test('shows the reason when starting work is refused', async () => {
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
})

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
  expect(mockStartInWorktree).toHaveBeenCalledWith('PROJ-999')
  expect(mockSwitchTo).not.toHaveBeenCalled()

  // Act: switch to it
  await user.click(within(offer).getByRole('button', { name: 'Switch to it' }))

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
          {
            name: 'feat/PROJ-999',
            issue_key: 'PROJ-999',
            current: false,
            worktree: '/home/ana/src/api-feat-PROJ-999',
            worktree_shown: '~/src/api-feat-PROJ-999',
          },
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
        { name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true },
        {
          name: 'feat/PROJ-2-metrics',
          issue_key: 'PROJ-2',
          current: false,
          worktree: '/home/ana/src/api-feat-PROJ-2-metrics',
          worktree_shown: '~/src/api-feat-PROJ-2-metrics',
        },
      ],
    }),
  })
  render(<WorkStory issueKey="PROJ-2" />)

  // Act
  await user.click(
    screen.getByRole('button', { name: 'Switch to its worktree, ~/src/api-feat-PROJ-2-metrics' }),
  )

  // Assert
  expect(screen.queryByRole('button', { name: 'Switch branch' })).toBeNull()
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

  // Act
  act(() => {
    useSnapshotStore.setState({
      status: 'live',
      snapshot: makeSnapshot({
        here: '/home/ana/src/api-feat-PROJ-999',
        branches: [{ name: 'feat/PROJ-999', issue_key: 'PROJ-999', current: true }],
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

  // Act
  await user.click(await screen.findByRole('button', { name: 'Switch to it' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toMatch(/a write is being made/)
})

test('a branch held by a worktree that is gone says how to free it', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [
        { name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true },
        {
          name: 'feat/PROJ-2-metrics',
          issue_key: 'PROJ-2',
          current: false,
          worktree: '/home/ana/src/api-gone',
          worktree_shown: '~/src/api-gone',
          worktree_missing: true,
        },
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

test('a worktree that could not be made says why', async () => {
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
})

test.each([
  ['not started', 'PROJ-999'],
  ['in flight elsewhere', 'PROJ-2'],
  ['on the checked-out branch', 'PROJ-1'],
])('names the merge request on GitLab for an issue %s', (_, issueKey) => {
  // Arrange
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  offHead()

  // Act
  render(<WorkStory issueKey={issueKey} />)

  // Assert
  expect(screen.getByText('Merge request')).toBeTruthy()
  expect(document.body.textContent).not.toMatch(/pull request/i)
})

test('marks the open merge request with its !number on GitLab', () => {
  // Arrange
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      review: {
        found: true,
        pull: {
          number: 7,
          url: 'https://x/7',
          title: 'the change',
          state: 'open',
          draft: false,
          approvals: 0,
          changes_requested: false,
          mergeable: 'unknown',
        },
      },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText('!7')).toBeTruthy()
})

test('renders nothing before a snapshot arrives', () => {
  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.queryByText('Branch')).toBeNull()
})

test.each([
  ['a channel', { service: 'Slack', configured: true, channel: '#dev' }, 'Announce to #dev'],
  [
    'a webhook of its own',
    { service: 'Teams', configured: true, channel: '' },
    'Announce to Teams',
  ],
  ['nothing set up', { service: 'Slack', configured: false, channel: '' }, 'Slack not configured'],
])('the Announce stage says where it would announce to, with %s', (_, destination, said) => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      messaging: { ...destination, channels: [], author: '' },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText(said)).toBeTruthy()
})
