import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Run, RunEvent, RunRequest, Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'
import { startRun, stopRun } from './gitRunApi.ts'

// titles are what the server titles each run.
const titles: Record<Run['kind'], string> = {
  pre_commit: 'pre-commit',
  rebase: 'git rebase',
  amend: 'git commit --amend',
  fixup: 'git commit --fixup',
}

// ended is a run as it ends, having written lines.
function ended(request: RunRequest, state: Run['state'], outcome: string, lines: string[]): Run {
  return { kind: request.kind, title: titles[request.kind], state, outcome, lines }
}

vi.mock('./gitRunApi.ts', () => ({
  startRun: vi.fn((request: RunRequest, onEvent: (event: RunEvent) => void) => {
    onEvent({
      run: {
        kind: request.kind,
        title: 'pre-commit',
        state: 'in_progress',
        outcome: '',
        lines: [],
      },
    })
    onEvent({ line: 'lint ok' })

    return Promise.resolve(ended(request, 'succeeded', 'The pre-commit hook passed.', ['lint ok']))
  }),
  stopRun: vi.fn(() => Promise.resolve()),
  readHookSetup: vi.fn(() =>
    Promise.resolve({ offered: false, hooks: [], config: '', scripts: 0 }),
  ),
  writeHookSetup: vi.fn(() => Promise.resolve()),
}))
const mockStartRun = vi.mocked(startRun)
const mockStopRun = vi.mocked(stopRun)

vi.mock('./stagingApi.ts', () => ({
  stageFile: vi.fn(() => Promise.resolve()),
  unstageFile: vi.fn(() => Promise.resolve()),
  stageEverything: vi.fn(() => Promise.resolve()),
  readDiff: vi.fn(() => Promise.resolve({ path: '', lines: [] })),
}))
vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve(makeBranch())) }))

// foldable streams a feature branch with two commits not yet pushed and a
// staged change, so every run is offered.
function foldable(overrides: Partial<Snapshot> = {}) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({
        upstream: '',
        commits: [
          { hash: 'h1h1h1h1', subject: 'feat: first', unpushed: true },
          { hash: 'h2h2h2h2', subject: 'fix: second', unpushed: true },
        ],
      }),
      changes: {
        changes: [
          { path: 'a.go', kind: 'modified', staged: true, has_unstaged: false, conflicted: false },
        ],
      },
      ...overrides,
    }),
  })
}

test('runs pre-commit at once and streams what it writes', async () => {
  // Arrange
  foldable()
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Run pre-commit' }))

  // Assert
  expect(mockStartRun).toHaveBeenLastCalledWith({ kind: 'pre_commit' }, expect.any(Function))
  const output = await screen.findByRole('region', { name: 'Output of pre-commit' })
  expect(output.textContent).toContain('lint ok')
  expect(screen.getByText('The pre-commit hook passed.')).toBeTruthy()
})

test('rebases only after a last look at what it rewrites', async () => {
  // Arrange
  foldable()
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Rebase onto main' }))
  const look = screen.getByRole('form', { name: 'Rebase fix/PROJ-1 onto main' })
  expect(mockStartRun).not.toHaveBeenCalled()

  // Act
  await user.click(within(look).getByRole('button', { name: 'Rebase' }))

  // Assert
  expect(mockStartRun).toHaveBeenLastCalledWith({ kind: 'rebase' }, expect.any(Function))
})

test('amends the last commit after a last look', async () => {
  // Arrange
  foldable()
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Amend last commit' }))
  const look = screen.getByRole('form', { name: 'Amend fix: second' })

  // Act
  await user.click(within(look).getByRole('button', { name: 'Amend' }))

  // Assert
  expect(mockStartRun).toHaveBeenLastCalledWith({ kind: 'amend' }, expect.any(Function))
})

test('fixes up the commit chosen', async () => {
  // Arrange
  foldable()
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Fix up a commit' }))
  const look = screen.getByRole('form', { name: 'Fix up a commit' })
  await user.click(within(look).getByRole('radio', { name: /feat: first/ }))

  // Act
  await user.click(within(look).getByRole('button', { name: 'Fix up' }))

  // Assert
  expect(mockStartRun).toHaveBeenLastCalledWith(
    { kind: 'fixup', commit: 'h1h1h1h1' },
    expect.any(Function),
  )
})

test('offers no amend or fixup with nothing staged', () => {
  // Arrange
  foldable({ changes: { changes: [] } })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Amend last commit' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Fix up a commit' })).toBeNull()
})

test('says why a run was refused, beside its output', async () => {
  // Arrange
  mockStartRun.mockImplementationOnce((request, onEvent) => {
    onEvent({ line: 'lint: 2 issues' })

    return Promise.resolve(
      ended(request, 'refused', 'The pre-commit hook failed.', ['lint: 2 issues']),
    )
  })
  foldable()
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Run pre-commit' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toContain('The pre-commit hook failed.')
  expect(screen.getByRole('region', { name: 'Output of pre-commit' }).textContent).toContain(
    'lint: 2 issues',
  )
})

test('stops the run going, from the stream, when asked', async () => {
  // Arrange
  foldable({
    run: { kind: 'rebase', title: 'git rebase', state: 'in_progress', outcome: '', lines: ['…'] },
  })
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Stop git rebase' }))

  // Assert
  expect(mockStopRun).toHaveBeenCalled()
})
