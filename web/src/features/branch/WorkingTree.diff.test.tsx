import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'
import { readDiff } from './stagingApi.ts'

vi.mock('./stagingApi.ts', () => ({
  stageFile: vi.fn(() => Promise.resolve()),
  unstageFile: vi.fn(() => Promise.resolve()),
  stageEverything: vi.fn(() => Promise.resolve()),
  readDiff: vi.fn(() =>
    Promise.resolve({ path: 'a.go', lines: ['--- a/a.go', '+++ b/a.go', '-old', '+new'] }),
  ),
}))
const mockReadDiff = vi.mocked(readDiff)

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve(makeBranch())) }))

beforeEach(() => {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      changes: {
        changes: [
          { path: 'a.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
        ],
      },
    }),
  })
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
