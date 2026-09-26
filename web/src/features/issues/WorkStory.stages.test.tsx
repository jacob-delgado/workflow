import { render, screen, within } from '@testing-library/react'
import type { Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { WorkStory } from './WorkStory.tsx'

// The story's stages follow the rules internal/progress writes for the
// terminal's spine and `workflow status`; each case here is named for the case
// of progress_test.go it matches, so the two stay equal.

// streamOnHead streams a snapshot in which PROJ-1 owns the checked-out branch,
// with the fields a case cares about overridden.
function streamOnHead(overrides: Partial<Snapshot>) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [{ name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true }],
      ...overrides,
    }),
  })
}

// storyStage reads one stage as a reader meets it: the mark drawn beside it and
// the state its button says in words.
function storyStage(title: string): { mark: string; state: string } {
  const button = screen.getByRole('button', { name: new RegExp(`^${title}`) })
  const state = within(button).getByText(/^(done|active|upcoming|failed)$/).textContent

  return { mark: markShape(button.parentElement ?? button), state }
}

test('reads the changes stage done on a commit with files still to commit, as progress does', () => {
  // Arrange
  streamOnHead({
    branch: makeBranch({ commits: [{ hash: 'h1h2h3h4', subject: 'do the work' }] }),
    changes: {
      changes: [
        { path: 'a.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
      ],
    },
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(storyStage('Changes')).toEqual({ mark: drawnMark('done'), state: 'done' })
})
