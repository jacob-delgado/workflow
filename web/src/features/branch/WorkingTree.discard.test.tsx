import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Change } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'
import { discardFile, unstageEverything } from './stagingApi.ts'

vi.mock('./stagingApi.ts', () => ({
  stageFile: vi.fn(() => Promise.resolve()),
  unstageFile: vi.fn(() => Promise.resolve()),
  stageEverything: vi.fn(() => Promise.resolve()),
  unstageEverything: vi.fn(() => Promise.resolve()),
  discardFile: vi.fn(() => Promise.resolve()),
  readDiff: vi.fn(() => Promise.resolve({ path: '', lines: [] })),
}))
const mockUnstageEverything = vi.mocked(unstageEverything)
const mockDiscardFile = vi.mocked(discardFile)

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve(makeBranch())) }))

// Unstaging everything and discarding a file: the one reversible, so it acts
// at once; the other not, so it asks first.

// change is a changed file as the stream lists it: an edit the index does not
// hold, unless a case says otherwise.
function change(path: string, overrides: Partial<Change> = {}): Change {
  return {
    path,
    kind: 'modified',
    staged: false,
    has_unstaged: true,
    conflicted: false,
    ...overrides,
  }
}

const wholly = { staged: true, has_unstaged: false }

// streamTree has the stream push a working tree of these changes.
function streamTree(changes: Change[]) {
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot({ changes: { changes } }) })
}

// unstageAll is the Unstage all button.
function unstageAll(): HTMLButtonElement {
  return screen.getByRole<HTMLButtonElement>('button', { name: 'Unstage all' })
}

test.each([
  ['nothing is staged', [change('a.go')], true],
  ['a file is staged', [change('a.go', wholly), change('b.go')], false],
])('Unstage all stands beside Stage all, off when %s', (_, changes, off) => {
  // Arrange
  streamTree(changes)

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Stage all' })).toBeTruthy()
  expect(unstageAll().disabled).toBe(off)
})

test('Unstage all acts at once and says so', async () => {
  // Arrange
  const user = userEvent.setup()
  streamTree([change('a.go', wholly), change('b.go', wholly)])
  render(<BranchPanel />)

  // Act
  await user.click(unstageAll())

  // Assert
  expect(await screen.findByText('Unstaged every change.')).toBeTruthy()
  expect(mockUnstageEverything).toHaveBeenCalledTimes(1)
})

test('says why the changes could not all be unstaged', async () => {
  // Arrange
  mockUnstageEverything.mockRejectedValueOnce({ code: 'unprocessable', detail: 'git refused' })
  const user = userEvent.setup()
  streamTree([change('a.go', wholly)])
  render(<BranchPanel />)

  // Act
  await user.click(unstageAll())

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('git refused')
})

test('Discard asks first, says what it costs, and discards only once confirmed', async () => {
  // Arrange
  const user = userEvent.setup()
  streamTree([change('a.go')])
  render(<BranchPanel />)

  // Act: ask to discard
  await user.click(screen.getByRole('button', { name: 'Discard a.go…' }))

  // Assert: the question has the focus, says it cannot be undone, and nothing
  // is discarded yet
  const question = screen.getByRole('group', { name: 'Discard the changes to a.go?' })
  expect(document.activeElement).toBe(question)
  expect(within(question).getByText(/cannot be undone/)).toBeTruthy()
  expect(mockDiscardFile).not.toHaveBeenCalled()

  // Act: confirm it
  await user.click(within(question).getByRole('button', { name: 'Discard' }))

  // Assert: that file is discarded, said, and the question gone
  expect(mockDiscardFile).toHaveBeenCalledWith('a.go')
  expect((await screen.findByText('Discarded a.go.')).getAttribute('role')).toBe('status')
  expect(screen.queryByRole('group', { name: 'Discard the changes to a.go?' })).toBeNull()
})

test('Cancel discards nothing and hands focus back', async () => {
  // Arrange
  const user = userEvent.setup()
  streamTree([change('a.go')])
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Discard a.go…' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(screen.queryByRole('group', { name: 'Discard the changes to a.go?' })).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Discard a.go…' }))
  expect(mockDiscardFile).not.toHaveBeenCalled()
})

test.each<[string, Change['kind'], RegExp]>([
  ['an untracked file', 'untracked', /the file is deleted/],
  ['a new file', 'new', /the file is deleted/],
  ['a modified file', 'modified', /goes back to the last commit/],
])('%s is said to be lost as it would be', async (_, kind, cost) => {
  // Arrange
  const user = userEvent.setup()
  streamTree([change('notes.txt', { kind })])
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Discard notes.txt…' }))

  // Assert
  const question = screen.getByRole('group', { name: 'Discard the changes to notes.txt?' })
  expect(within(question).getByText(cost)).toBeTruthy()
})

test('a refused discard says why and keeps the question', async () => {
  // Arrange
  mockDiscardFile.mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'git would not discard a.go',
  })
  const user = userEvent.setup()
  streamTree([change('a.go')])
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Discard a.go…' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Discard' }))

  // Assert
  const question = screen.getByRole('group', { name: 'Discard the changes to a.go?' })
  expect((await within(question).findByRole('alert')).textContent).toBe(
    'git would not discard a.go',
  )
})
