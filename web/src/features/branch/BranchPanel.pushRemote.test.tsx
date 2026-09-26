import { render, screen } from '@testing-library/react'
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
        commits: [{ hash: 'c0ffee1', subject: 'fix: redact tokens' }],
      }),
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Push branch' })).toBeTruthy()
})
