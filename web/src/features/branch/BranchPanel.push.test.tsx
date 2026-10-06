import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
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
