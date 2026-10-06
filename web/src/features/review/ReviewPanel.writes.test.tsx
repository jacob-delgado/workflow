import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Ci, PullRequest } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'
import {
  editPull,
  finishBranch,
  mergePull,
  readMergeOffer,
  readPullText,
  rerunChecks,
} from './reviewWritesApi.ts'

// ready is #42, open, approved, clean.
const ready: PullRequest = {
  number: 42,
  url: 'https://forge.example.com/pull/42',
  title: 'Redact tokens',
  state: 'open',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean',
}

vi.mock('./reviewWritesApi.ts', () => ({
  readPullText: vi.fn(() => Promise.resolve({ title: 'Redact tokens', body: 'Why: they leak.' })),
  editPull: vi.fn((title: string) => Promise.resolve({ ...ready, title })),
  readMergeOffer: vi.fn(() => Promise.resolve({ pull: ready, methods: ['squash', 'rebase'] })),
  mergePull: vi.fn(() => Promise.resolve({ ...ready, state: 'merged' })),
  finishBranch: vi.fn(() => Promise.resolve(makeBranch({ name: 'main' }))),
  rerunChecks: vi.fn(() => Promise.resolve({ reran: true })),
}))

// ciOf is CI in one state, with one check that keeps a log.
function ciOf(state: Ci['state']): Ci {
  return {
    state,
    total: 1,
    done: 1,
    failed: state === 'failed' ? 1 : 0,
    checks: [{ name: 'build', state, url: '', id: '7', log_available: true }],
  }
}

// streamPull streams the branch's pull request and its CI.
function streamPull(pull: PullRequest, ci?: Ci) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({ name: 'fix/PROJ-1', ahead: 0 }),
      review: { found: true, announced: false, pull, ci },
    }),
  })
}

test('edits the pull request from a form that is its last look', async () => {
  // Arrange
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Edit pull request' }))
  const form = await screen.findByRole('form', { name: 'Edit #42' })
  const title = within(form).getByRole('textbox', { name: 'Title' })
  await user.clear(title)
  await user.type(title, 'Redact every token')

  // Act
  await user.click(within(form).getByRole('button', { name: 'Save' }))

  // Assert
  expect(vi.mocked(editPull)).toHaveBeenCalledWith('Redact every token', 'Why: they leak.')
  expect(await screen.findByText('Saved #42.')).toBeTruthy()
})

test('merges by the method chosen among those permitted, after a preview', async () => {
  // Arrange
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Merge' }))
  const form = await screen.findByRole('form', { name: 'Merge #42' })
  await user.click(within(form).getByRole('radio', { name: 'Rebase and merge' }))
  expect(vi.mocked(mergePull)).not.toHaveBeenCalled()

  // Act
  await user.click(within(form).getByRole('button', { name: 'Merge' }))

  // Assert
  expect(vi.mocked(mergePull)).toHaveBeenCalledWith('rebase')
  expect(vi.mocked(readMergeOffer)).toHaveBeenCalled()
})

test('offers no merge until the pull request can be merged', () => {
  // Arrange
  streamPull({ ...ready, approvals: 0 }, ciOf('passed'))

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Merge' })).toBeNull()
})

test('finishes a merged branch after a look at the commands it runs', async () => {
  // Arrange
  streamPull({ ...ready, state: 'merged' })
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Finish the branch' }))
  const form = screen.getByRole('form', { name: 'Finish fix/PROJ-1' })
  expect(form.textContent).toContain('git branch -D fix/PROJ-1')

  // Act
  await user.click(within(form).getByRole('button', { name: 'Finish' }))

  // Assert
  expect(vi.mocked(finishBranch)).toHaveBeenCalled()
  expect(await screen.findByText('Finished fix/PROJ-1; now on main.')).toBeTruthy()
})

test('re-runs failed CI after a last look', async () => {
  // Arrange
  streamPull(ready, ciOf('failed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Re-run failed checks' }))
  const form = screen.getByRole('form', { name: 'Re-run the failed checks on #42' })

  // Act
  await user.click(within(form).getByRole('button', { name: 'Re-run' }))

  // Assert
  expect(vi.mocked(rerunChecks)).toHaveBeenCalled()
  expect(await screen.findByText('Re-ran the failed checks on #42.')).toBeTruthy()
})

test('reads the log of a check that passed, not only a failed one', () => {
  // Arrange
  streamPull(ready, ciOf('passed'))

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Show log of build' })).toBeTruthy()
})

test('says why an edit was refused, keeping the form', async () => {
  // Arrange
  vi.mocked(readPullText).mockResolvedValueOnce({ title: 'x', body: '' })
  vi.mocked(editPull).mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'GitHub refused this',
  })
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Edit pull request' }))
  const form = await screen.findByRole('form', { name: 'Edit #42' })

  // Act
  await user.click(within(form).getByRole('button', { name: 'Save' }))

  // Assert
  expect((await within(form).findByRole('alert')).textContent).toContain('GitHub refused this')
})
