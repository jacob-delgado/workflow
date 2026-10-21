import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'

test('offers a push for a branch level with an upstream off the remote its push goes to', () => {
  // Arrange
  // remote.pushDefault sends the push to fork, which does not hold the branch
  // its upstream on origin is level with.
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({
        upstream: 'origin/fix/PROJ-1',
        push_remote: 'fork',
        base_remote: 'origin',
        ahead: 0,
        commits: [{ hash: 'c0ffee1', subject: 'fix: redact tokens', unpushed: false }],
      }),
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Push branch' })).toBeTruthy()
})

// unpublished is a branch never pushed, whose base could not be found: the
// commits on it since a base cannot be counted.
function unpublished() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({
        name: 'fix/PROJ-1',
        commits: [],
        upstream: '',
        ahead: 0,
        behind: 0,
        base: '',
      }),
    }),
  })
}

test('the push confirm names the branch and its remote, and no count', async () => {
  // Arrange
  unpublished()
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))

  // Assert
  expect(screen.getByRole('group', { name: 'Push fix/PROJ-1 to origin?' })).toBeTruthy()
})

test('an unpublished branch is not pushed yet, with no counts against an upstream it lacks', () => {
  // Arrange
  unpublished()

  // Act
  render(<BranchPanel />)

  // Assert
  const summary = screen.getByRole('region', { name: 'fix/PROJ-1' })
  expect(within(summary).getByText('not pushed yet')).toBeTruthy()
  expect(within(summary).queryByText(/ahead/)).toBeNull()
})

test('with no base, the commits are unknown rather than none', () => {
  // Arrange
  unpublished()

  // Act
  render(<BranchPanel />)

  // Assert
  const commits = screen.getByRole('region', { name: 'Commits' })
  expect(within(commits).getByText(/base branch is unknown/)).toBeTruthy()
  expect(screen.queryByText(/No commits yet/)).toBeNull()
})

test('a published branch counts how far it is ahead and behind its upstream', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ branch: makeBranch({ ahead: 2, behind: 1 }) }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByText('2 ahead, 1 behind')).toBeTruthy()
})

test('offers no push on the base branch itself, however far ahead it is', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({ name: 'main', base: 'origin/main', upstream: 'origin/main', ahead: 1 }),
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Push branch' })).toBeNull()
})

// forkBranch is a fork's clone's branch: its base is upstream's, its push
// goes to pushRemote.
function forkBranch(pushRemote: string) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({
        base: 'upstream/main',
        base_remote: 'upstream',
        push_remote: pushRemote,
      }),
    }),
  })
}

test("names the push remote when it is not the base's", () => {
  // Arrange
  forkBranch('origin')

  // Act
  render(<BranchPanel />)

  // Assert
  const term = screen.getAllByRole('term').find((shown) => shown.textContent === 'Push to')
  expect(term?.nextElementSibling?.textContent).toBe('origin')
})

test("names no push remote when it is the base's", () => {
  // Arrange
  forkBranch('upstream')

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getAllByRole('term').map((shown) => shown.textContent)).not.toContain('Push to')
})

test('Pushed is said only once the branch no longer reads ahead', async () => {
  // Arrange
  const user = userEvent.setup()
  const pushed = makeBranch({ ahead: 0 })
  fakeApi({ '/api/push': pushed }, { frames: 'held' })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ branch: makeBranch({ ahead: 1 }) }),
  })
  render(<BranchPanel />)

  // Act: push, with the frame the push woke not yet landed
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: /^push$/i }))
  await new Promise((resolve) => setTimeout(resolve, 0))

  // Assert: nothing says the branch went while it still reads ahead
  expect(screen.getByText('1 ahead, 0 behind')).toBeTruthy()
  expect(screen.queryByText('Pushed fix/PROJ-1.')).toBeNull()

  // Act: the frame lands
  act(() => {
    pushFrame(makeSnapshot({ branch: pushed }))
  })

  // Assert: the notice stands beside the branch up to date
  await screen.findByText('Pushed fix/PROJ-1.')
  expect(screen.getByText('0 ahead, 0 behind')).toBeTruthy()
})
