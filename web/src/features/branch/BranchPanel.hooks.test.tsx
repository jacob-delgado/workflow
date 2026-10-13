import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { ConfiguredHooks, RunEvent, RunRequest } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useKeysStore } from '@/features/keyboard/keysApi.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'
import { readHooks, startRun } from './gitRunApi.ts'

// Run a hook in the Branch section: the hooks lefthook configures, read when
// it is asked for, the one chosen run at once as the terminal's h runs it, and
// the hooks only git runs listed with why.

// configured is a lefthook configuration of two hooks a run by hand can run
// and one only git runs.
const configured: ConfiguredHooks = {
  runnable: ['pre-commit', 'pre-push'],
  git_only: [{ name: 'commit-msg', reason: 'git hands it the file holding the commit message' }],
}

vi.mock('./gitRunApi.ts', () => ({
  readHooks: vi.fn(() => Promise.resolve(configured)),
  startRun: vi.fn((request: RunRequest, onEvent: (event: RunEvent) => void) => {
    const title = request.hook ?? ''
    onEvent({ run: { kind: request.kind, title, state: 'in_progress', outcome: '', lines: [] } })
    onEvent({ line: 'lint ok' })

    return Promise.resolve({
      kind: request.kind,
      title,
      state: 'succeeded',
      outcome: `The ${title} hook passed.`,
      lines: ['lint ok'],
    })
  }),
  stopRun: vi.fn(() => Promise.resolve()),
  readHookSetup: vi.fn(() =>
    Promise.resolve({ offered: false, hooks: [], config: '', scripts: 0 }),
  ),
  writeHookSetup: vi.fn(() => Promise.resolve()),
}))
const mockReadHooks = vi.mocked(readHooks)
const mockStartRun = vi.mocked(startRun)

vi.mock('./stagingApi.ts', () => ({
  stageFile: vi.fn(() => Promise.resolve()),
  unstageFile: vi.fn(() => Promise.resolve()),
  stageEverything: vi.fn(() => Promise.resolve()),
  unstageEverything: vi.fn(() => Promise.resolve()),
  discardFile: vi.fn(() => Promise.resolve()),
  readDiff: vi.fn(() => Promise.resolve({ path: '', lines: [] })),
}))
vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve(makeBranch())) }))

// streaming is the Branch section live on a feature branch with a staged
// change.
function streaming() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({ upstream: '' }),
      changes: {
        changes: [
          { path: 'a.go', kind: 'modified', staged: true, has_unstaged: false, conflicted: false },
        ],
      },
    }),
  })
}

// opened is the Branch section with Run a hook asked for, and its chooser.
async function opened() {
  streaming()
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Run a hook' }))

  return { user, chooser: await screen.findByRole('form', { name: 'Run a hook' }) }
}

test('Run a hook offers the hooks lefthook runs by hand, pre-commit chosen first', async () => {
  // Act
  const { chooser } = await opened()

  // Assert
  const choices = within(chooser).getAllByRole('radio')
  expect(choices.map((choice) => choice.getAttribute('value'))).toEqual(['pre-commit', 'pre-push'])
  expect(within(chooser).getByRole('radio', { name: 'pre-commit' })).toHaveProperty('checked', true)
  expect(mockStartRun).not.toHaveBeenCalled()
})

test('runs the hook chosen and streams what it writes', async () => {
  // Arrange
  const { user, chooser } = await opened()
  await user.click(within(chooser).getByRole('radio', { name: 'pre-push' }))

  // Act
  await user.click(within(chooser).getByRole('button', { name: 'Run' }))

  // Assert
  expect(mockStartRun).toHaveBeenLastCalledWith(
    { kind: 'hook', hook: 'pre-push' },
    expect.any(Function),
  )
  const output = await screen.findByRole('region', { name: 'Output of pre-push' })
  expect(output.textContent).toContain('lint ok')
  expect(screen.queryByRole('form', { name: 'Run a hook' })).toBeNull()
})

test('lists the hooks only git runs with what git hands each, and offers none of them as a choice', async () => {
  // Act
  const { chooser } = await opened()

  // Assert
  const gitOnly = within(chooser).getByRole('list', { name: 'Hooks only git runs' })
  expect(within(gitOnly).getByRole('listitem').textContent).toBe(
    'commit-msg: git hands it the file holding the commit message',
  )
  expect(within(chooser).queryByRole('radio', { name: 'commit-msg' })).toBeNull()
})

test('says no hook can be run by hand, and Run stays off', async () => {
  // Arrange
  mockReadHooks.mockResolvedValueOnce({ runnable: [], git_only: configured.git_only })

  // Act
  const { chooser } = await opened()

  // Assert
  expect(chooser.textContent).toContain("None of lefthook's hooks here can be run by hand.")
  expect(within(chooser).getByRole('button', { name: 'Run' })).toHaveProperty('disabled', true)
})

test('says lefthook configures no hook, and Run stays off', async () => {
  // Arrange
  mockReadHooks.mockResolvedValueOnce({ runnable: [], git_only: [] })

  // Act
  const { chooser } = await opened()

  // Assert
  expect(chooser.textContent).toContain('lefthook configures no hook here.')
  expect(within(chooser).queryByRole('list', { name: 'Hooks only git runs' })).toBeNull()
  expect(within(chooser).getByRole('button', { name: 'Run' })).toHaveProperty('disabled', true)
})

test('says why the hooks could not be read, beside Run a hook', async () => {
  // Arrange
  mockReadHooks.mockRejectedValueOnce({
    code: 'unprocessable',
    detail:
      "lefthook's configuration could not be read; run lefthook dump in a terminal to see why",
  })
  streaming()
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Run a hook' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toContain(
    "lefthook's configuration could not be read",
  )
  expect(screen.queryByRole('form', { name: 'Run a hook' })).toBeNull()
})

test('Cancel puts the choice away and hands focus back to Run a hook', async () => {
  // Arrange
  const { user, chooser } = await opened()

  // Act
  await user.click(within(chooser).getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(screen.queryByRole('form', { name: 'Run a hook' })).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Run a hook' }))
  expect(mockStartRun).not.toHaveBeenCalled()
})

test('Run a hook carries the run-hook key', () => {
  // Arrange
  useKeysStore.setState({
    shortcuts: true,
    actions: [
      {
        action: 'run-hook',
        help: 'run a hook',
        group: 'Commits',
        shown: 'h',
        keys: ['h'],
        default: 'h',
      },
    ],
  })
  streaming()

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Run a hook' }).getAttribute('aria-keyshortcuts')).toBe(
    'h',
  )
})
