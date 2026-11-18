import { render, screen } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'

test('names a stopped rebase and the branch it replays', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({
        name: '',
        detached: true,
        head: 'abcdef1234',
        operation: 'rebase',
        rebasing: 'fix/PROJ-1',
      }),
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  const heading = screen.getByRole('heading', { name: 'Rebasing fix/PROJ-1' })
  expect(heading.closest('section')?.textContent).toMatch(
    'A rebase of fix/PROJ-1 is stopped part way. Resolve what it stopped on, then run git rebase --continue, or git rebase --abort to go back.',
  )
})

test('says to continue or abort a stopped cherry-pick', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ branch: makeBranch({ operation: 'cherry_pick' }) }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  const heading = screen.getByRole('heading', { name: 'fix/PROJ-1 · cherry-pick stopped' })
  expect(heading.closest('section')?.textContent).toMatch(
    'A cherry-pick is stopped part way. Resolve what it stopped on, then run git cherry-pick --continue, or git cherry-pick --abort to go back.',
  )
})
