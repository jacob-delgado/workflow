import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PullRequest } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

// Finishing a merged branch: the server switches to the base and wakes the
// stream as it answers, and the page hands the answer on only once the frame
// it woke has landed, which already shows the base checked out and starts its
// review afresh.

const merged: PullRequest = {
  number: 12,
  url: 'https://forge.example.com/pull/12',
  title: 'Redact tokens',
  state: 'merged',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'unknown',
}

// onBase is what the frame the finish woke shows: the base checked out, with
// no pull request of its own.
const onBase = makeSnapshot({
  branch: makeBranch({ name: 'main' }),
  review: { found: false, announced: false },
})

// finishing opens the finish's last look on fix/PROJ-1, whose pull request
// merged, with the frame its finish wakes held back.
async function finishing() {
  const user = userEvent.setup()
  fakeApi({ '/api/branch/finish': makeBranch({ name: 'main' }) }, { frames: 'held' })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({ name: 'fix/PROJ-1', ahead: 0, finish_commands: ['git switch main'] }),
      review: { found: true, announced: false, pull: merged },
    }),
  })
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Finish the branch' }))
  const form = screen.getByRole('form', { name: 'Finish fix/PROJ-1' })
  await user.click(within(form).getByRole('button', { name: 'Finish' }))
}

test('a finish says it finished once the frame showing the base has landed', async () => {
  // Arrange
  await finishing()

  // Act
  act(() => {
    pushFrame(onBase)
  })

  // Assert
  expect(await screen.findByText('Finished fix/PROJ-1; now on main.')).toBeTruthy()
})

test('a finish hands focus to what it said, its button gone with the branch', async () => {
  // Arrange
  await finishing()

  // Act
  act(() => {
    pushFrame(onBase)
  })

  // Assert
  const said = await screen.findByText('Finished fix/PROJ-1; now on main.')
  expect(document.activeElement).toBe(said)
})

test('what a finish said goes once another branch is checked out', async () => {
  // Arrange
  await finishing()
  act(() => {
    pushFrame(onBase)
  })
  await screen.findByText('Finished fix/PROJ-1; now on main.')

  // Act
  act(() => {
    pushFrame(makeSnapshot({ branch: makeBranch({ name: 'feat/PROJ-2' }) }))
  })

  // Assert
  expect(screen.queryByText('Finished fix/PROJ-1; now on main.')).toBeNull()
})
