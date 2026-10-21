import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Run } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'

// A git run's end beside the branch it left: the server wakes the stream
// before it sends how the run ended, and the page says how it ended only once
// the frame that end woke has landed.

// rebase is the rebase as its stream says it went: under way, then ended.
function rebase(state: Run['state'], outcome = ''): Run {
  return { kind: 'rebase', title: 'rebase onto main', state, outcome, lines: [] }
}

// rebasing is the server's streamed answer to a rebase that succeeds.
function rebasing(): Response {
  const events = [
    { run: rebase('in_progress') },
    { run: rebase('succeeded', 'Rebased onto main.') },
  ]

  return new Response(events.map((event) => JSON.stringify(event)).join('\n') + '\n', {
    headers: { 'Content-Type': 'application/x-ndjson' },
  })
}

test('a rebase says how it ended only once the stream has pushed a frame after it', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ '/api/runs': () => rebasing() }, { frames: 'held' })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ branch: makeBranch({ upstream: '', behind: 1 }) }),
  })
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Rebase onto main' }))
  const look = screen.getByRole('form', { name: 'Rebase fix/PROJ-1 onto main' })

  // Act: rebase, with the frame the run's end woke not yet landed
  await user.click(within(look).getByRole('button', { name: 'Rebase' }))
  await new Promise((resolve) => setTimeout(resolve, 0))

  // Assert: the run does not say it ended yet
  expect(screen.queryByText('Rebased onto main.')).toBeNull()

  // Act: the frame lands
  act(() => {
    pushFrame(makeSnapshot({ branch: makeBranch({ upstream: '', behind: 0 }) }))
  })

  // Assert: now it does
  expect(await screen.findByText('Rebased onto main.')).toBeTruthy()
})
