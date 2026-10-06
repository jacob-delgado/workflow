import { render, screen } from '@testing-library/react'
import type { Problem } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { BranchPanel } from './BranchPanel.tsx'

// gitFailed is the problem the server sends for a git read it cannot class.
const gitFailed: Problem = {
  type: 'https://jacob-delgado.github.io/workflow/docs/errors/#internal',
  title: 'Internal error',
  status: 500,
  detail:
    'the request could not be completed; try again, and run workflow doctor if it keeps failing',
  code: 'internal',
}

const noBranch = { ...makeBranch(), name: '', detached: false, head: '', base: '', commits: [] }

test('says the branch could not be read, rather than that this is no repository', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ branch: noBranch, problems: { branch: gitFailed } }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  const alert = screen.getByRole('alert')
  expect(alert.textContent).toBe(`The branch could not be read: ${gitFailed.detail}`)
  expect(markShape(alert.parentElement ?? alert)).toBe(drawnMark('failed'))
  expect(screen.queryByText(/not a Git repository/)).toBeNull()
})

test('says the working tree could not be read, rather than that it is clean', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ problems: { changes: gitFailed } }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('alert').textContent).toBe(
    `The working tree could not be read: ${gitFailed.detail}`,
  )
  expect(screen.queryByText(/Clean — nothing to commit/)).toBeNull()
})
