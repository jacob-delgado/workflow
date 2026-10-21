import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Change } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'

// A write's notice beside the working tree it changed: the server wakes the
// stream as it answers, and the page hands the answer on only once the frame
// it woke has landed, so the notice never stands beside rows that say
// otherwise.

// edited is a file with edits the index does not hold, or, staged, wholly in
// the index.
function edited(path: string, staged = false): Change {
  return { path, kind: 'modified', staged, has_unstaged: !staged, conflicted: false }
}

test('Stage all says it staged every change only beside rows that show them staged', async () => {
  // Arrange
  const user = userEvent.setup()
  const before = [edited('a.go'), edited('b.go')]
  const after = [edited('a.go', true), edited('b.go', true)]
  fakeApi({ '/api/stage': { changes: after } }, { frames: 'held' })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ changes: { changes: before } }),
  })
  render(<BranchPanel />)

  // Act: stage all, with the frame the stage woke not yet landed
  await user.click(screen.getByRole('button', { name: 'Stage all' }))
  await new Promise((resolve) => setTimeout(resolve, 0))

  // Assert: the stage is still running, and says nothing yet
  expect(screen.queryByText('Staged every change.')).toBeNull()
  expect(screen.getByRole('button', { name: /^Staging/ }).getAttribute('aria-disabled')).toBe(
    'true',
  )

  // Act: the frame lands
  act(() => {
    pushFrame(makeSnapshot({ changes: { changes: after } }))
  })

  // Assert: the notice stands beside rows that show every change staged
  await screen.findByText('Staged every change.')
  expect(screen.getByRole('button', { name: 'Unstage a.go' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Unstage b.go' })).toBeTruthy()
})
