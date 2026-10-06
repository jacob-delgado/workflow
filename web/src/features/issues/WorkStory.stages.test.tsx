import { render, screen, within } from '@testing-library/react'
import type { Ci, CiState, PullRequest, Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import type { MarkState } from '@/shell/StateMark.tsx'
import { WorkStory } from './WorkStory.tsx'

// The story's stages follow the rules internal/progress writes for the
// terminal's spine and `workflow status`, and the two are kept equal by hand:
// a case here that has a twin in progress_test.go carries its name. A CI not
// read counts there as none, and a draft has no twin, since progress.Work
// carries no draft.

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
    branch: makeBranch({
      commits: [{ hash: 'h1h2h3h4', subject: 'do the work', unpushed: false }],
    }),
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

// An open pull request, ready for review, with nothing asked of it.
const openPull: PullRequest = {
  number: 42,
  url: 'https://forge.example.com/pull/42',
  title: 'the change',
  state: 'open',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'unknown',
}

const mergedPull: PullRequest = { ...openPull, state: 'merged' }

// ciOf is CI in one state, reported whole, as the story reads only its state.
function ciOf(state: CiState): Ci {
  return { state, total: 0, done: 0, failed: 0, checks: [] }
}

// How the review stage reads, by progress.reviewState: in flight until CI
// passes, failed on a CI failure or changes asked for, done once CI passes or
// the pull request merges, and as no pull request at all once it is closed.
// A forge with no CI sends a CI state of none; a CI read that failed sends no
// CI at all. The story is at the review stage in every case, so in flight
// reads active.
test.each<{ name: string; pull: PullRequest; ci?: Ci; state: string; mark: MarkState }>([
  {
    name: 'committed, review open, CI running',
    pull: openPull,
    ci: ciOf('running'),
    state: 'active',
    mark: 'in-flight',
  },
  {
    name: 'review open, no CI to pass',
    pull: openPull,
    ci: ciOf('none'),
    state: 'active',
    mark: 'in-flight',
  },
  { name: 'review open, CI not read', pull: openPull, state: 'active', mark: 'in-flight' },
  { name: 'CI passed', pull: openPull, ci: ciOf('passed'), state: 'done', mark: 'done' },
  { name: 'CI failed', pull: openPull, ci: ciOf('failed'), state: 'failed', mark: 'failed' },
  {
    name: 'changes requested, with no CI, stop review reading done',
    pull: { ...openPull, changes_requested: true },
    ci: ciOf('none'),
    state: 'failed',
    mark: 'failed',
  },
  {
    name: 'changes requested, CI not read, stop review reading done',
    pull: { ...openPull, changes_requested: true },
    state: 'failed',
    mark: 'failed',
  },
  {
    name: 'changes requested stops review reading done',
    pull: { ...openPull, changes_requested: true },
    ci: ciOf('passed'),
    state: 'failed',
    mark: 'failed',
  },
  {
    name: 'a draft whose CI passed, as any pull request',
    pull: { ...openPull, draft: true },
    ci: ciOf('passed'),
    state: 'done',
    mark: 'done',
  },
  { name: 'merged, with no CI left to pass', pull: mergedPull, state: 'done', mark: 'done' },
  {
    name: 'merged, the changes once asked for moot',
    pull: { ...mergedPull, changes_requested: true },
    state: 'done',
    mark: 'done',
  },
  {
    name: 'closed without merging, none',
    pull: { ...openPull, state: 'closed', changes_requested: true },
    state: 'active',
    mark: 'in-flight',
  },
])('reads the review stage as progress does: $name', ({ pull, ci, state, mark }) => {
  // Arrange
  streamOnHead({
    branch: makeBranch({
      commits: [{ hash: 'h1h2h3h4', subject: 'do the work', unpushed: false }],
    }),
    review: { found: true, announced: false, pull, ci },
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(storyStage('Pull request')).toEqual({ mark: drawnMark(mark), state })
})

test('reads the announce stage done once the pull request was announced at its moment', () => {
  // Arrange
  streamOnHead({
    branch: makeBranch({
      commits: [{ hash: 'h1h2h3h4', subject: 'do the work', unpushed: false }],
    }),
    review: { found: true, pull: openPull, ci: ciOf('passed'), announced: true },
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(storyStage('Announce')).toEqual({ mark: drawnMark('done'), state: 'done' })
})
