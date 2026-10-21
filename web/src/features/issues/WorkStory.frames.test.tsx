import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot, makeTaskBranch } from '@/test/fixtures.ts'
import { WorkStory } from './WorkStory.tsx'

// Starting work beside the story it changes: the server wakes the stream as it
// answers, and the page hands the answer on only once the frame it woke has
// landed, so the notice never stands beside a story that has no branch yet.

test('Started work is said only once the story names the branch', async () => {
  // Arrange
  const user = userEvent.setup()
  const started = makeBranch({ name: 'fix/PROJ-999-redact' })
  fakeApi({ '/api/branches': started }, { frames: 'held' })
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-999" />)

  // Act: start work, with the frame the start woke not yet landed
  await user.click(screen.getByRole('button', { name: 'Start work' }))
  await new Promise((resolve) => setTimeout(resolve, 0))

  // Assert: nothing says work started while the story has no branch for it
  expect(screen.queryByText('Started work on fix/PROJ-999-redact.')).toBeNull()
  expect(screen.getByText(/appear here once you start work on it\./i)).toBeTruthy()

  // Act: the frame lands
  act(() => {
    pushFrame(
      makeSnapshot({
        branch: started,
        branches: [
          makeTaskBranch({ name: 'fix/PROJ-999-redact', issue_key: 'PROJ-999', current: true }),
        ],
      }),
    )
  })

  // Assert: the notice stands beside a story that names the branch
  await screen.findByText('Started work on fix/PROJ-999-redact.')
  expect(screen.queryByText(/appear here once you start work on it\./i)).toBeNull()
})
