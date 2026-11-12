import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import type { Change } from '@/api/generated/types.gen.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'
import { readDiff } from './stagingApi.ts'

vi.mock('./stagingApi.ts', () => ({
  stageFile: vi.fn(() => Promise.resolve()),
  unstageFile: vi.fn(() => Promise.resolve()),
  stageEverything: vi.fn(() => Promise.resolve()),
  unstageEverything: vi.fn(() => Promise.resolve()),
  discardFile: vi.fn(() => Promise.resolve()),
  readDiff: vi.fn(() =>
    Promise.resolve({ path: 'a.go', lines: ['--- a/a.go', '+++ b/a.go', '-old', '+new'] }),
  ),
}))
const mockReadDiff = vi.mocked(readDiff)

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve(makeBranch())) }))

const unstagedEdit: Change = {
  path: 'a.go',
  kind: 'modified',
  staged: false,
  has_unstaged: true,
  conflicted: false,
}

// showWith puts a.go, as change says it stands, on a branch at head.
function showWith(change: Change, head = 'abc1234') {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ branch: makeBranch({ head }), changes: { changes: [change] } }),
  })
}

// openDiff shows a.go's diff and waits for it.
async function openDiff(user: ReturnType<typeof userEvent.setup>) {
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Show diff of a.go' }))
  await screen.findByRole('region', { name: 'Diff of a.go' })
  mockReadDiff.mockClear()
}

beforeEach(() => {
  mockReadDiff.mockReset()
  showWith(unstagedEdit)
})

test('reads a changed file’s diff when asked, and not before', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Show diff of a.go' }))

  // Assert
  const diff = await screen.findByRole('region', { name: 'Diff of a.go' })
  expect(diff.textContent).toContain('+new')
  expect(mockReadDiff).toHaveBeenCalledWith('a.go')
})

test('hides the diff again, handing focus back to its button', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Show diff of a.go' }))
  await screen.findByRole('region', { name: 'Diff of a.go' })

  // Act
  await user.click(screen.getByRole('button', { name: 'Hide diff of a.go' }))

  // Assert
  expect(screen.queryByRole('region', { name: 'Diff of a.go' })).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Show diff of a.go' }))
})

test('says why a diff could not be read', async () => {
  // Arrange
  mockReadDiff.mockRejectedValueOnce({
    code: 'not_found',
    detail: 'the working tree lists no change at that path',
  })
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Show diff of a.go' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toContain('lists no change')
})

test('reads a shown diff again when its file’s change moves', async () => {
  // Arrange
  const user = userEvent.setup()
  await openDiff(user)
  mockReadDiff.mockResolvedValueOnce({ path: 'a.go', lines: ['+staged now'] })

  // Act
  act(() => {
    showWith({ ...unstagedEdit, staged: true, has_unstaged: false })
  })

  // Assert
  const diff = await screen.findByRole('region', { name: 'Diff of a.go' })
  await vi.waitFor(() => {
    expect(diff.textContent).toContain('+staged now')
  })
})

test('reads a shown diff again when the branch head moves', async () => {
  // Arrange
  const user = userEvent.setup()
  await openDiff(user)
  mockReadDiff.mockResolvedValueOnce({ path: 'a.go', lines: ['+after the commit'] })

  // Act
  act(() => {
    showWith(unstagedEdit, 'def5678')
  })

  // Assert
  await vi.waitFor(() => {
    expect(screen.getByRole('region', { name: 'Diff of a.go' }).textContent).toContain(
      '+after the commit',
    )
  })
})

test('keeps the old diff on screen while it reads again', async () => {
  // Arrange
  const user = userEvent.setup()
  await openDiff(user)
  mockReadDiff.mockReturnValueOnce(new Promise(() => undefined))

  // Act
  act(() => {
    showWith({ ...unstagedEdit, kind: 'renamed', original_path: 'old.go' })
  })

  // Assert
  expect(mockReadDiff).toHaveBeenCalledWith('a.go')
  expect(screen.getByRole('region', { name: 'Diff of a.go' }).textContent).toContain('+new')
})

test('leaves a shown diff be while its file stands as it did', async () => {
  // Arrange
  const user = userEvent.setup()
  await openDiff(user)

  // Act
  act(() => {
    showWith({ ...unstagedEdit })
  })

  // Assert
  expect(mockReadDiff).not.toHaveBeenCalled()
})

test('says why a diff could not be read again, beside the one it shows', async () => {
  // Arrange
  const user = userEvent.setup()
  await openDiff(user)
  mockReadDiff.mockRejectedValueOnce({ code: 'internal', detail: 'git diff exited 128' })

  // Act
  act(() => {
    showWith(unstagedEdit, 'def5678')
  })

  // Assert
  expect((await screen.findByRole('alert')).textContent).toContain('exited 128')
  expect(screen.getByRole('region', { name: 'Diff of a.go' }).textContent).toContain('+new')
})

test('a diff hidden while it reads again stays hidden', async () => {
  // Arrange
  const user = userEvent.setup()
  await openDiff(user)
  let answer: (diff: { path: string; lines: string[] }) => void = () => undefined
  mockReadDiff.mockReturnValueOnce(
    new Promise((resolve) => {
      answer = resolve
    }),
  )
  act(() => {
    showWith(unstagedEdit, 'def5678')
  })
  await user.click(screen.getByRole('button', { name: 'Hide diff of a.go' }))

  // Act
  await act(async () => {
    answer({ path: 'a.go', lines: ['+late'] })
    await Promise.resolve()
  })

  // Assert
  expect(screen.queryByRole('region', { name: 'Diff of a.go' })).toBeNull()
})
