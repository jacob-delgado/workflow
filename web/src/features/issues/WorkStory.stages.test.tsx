import { render, screen, within } from '@testing-library/react'
import type { Change, Ci, PullRequest, Snapshot, Stage } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot, makeStages } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import type { MarkState } from '@/lib/StateMark.tsx'
import { WorkStory } from './WorkStory.tsx'

// The story's stages for the checked-out branch are the server's, in its order
// and in the states it read them in, by the rules internal/progress writes for
// the terminal's spine and `workflow status`: the story draws whatever stages
// arrive, and works out none. Those rules are pinned in Go, by
// internal/webserver/stages_test.go.

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

// storyTitles is each stage's title, in the order the story lists them.
function storyTitles(): string[] {
  return screen
    .getAllByRole('listitem')
    .map((stage) => within(stage).getByRole('button').firstChild?.textContent ?? '')
}

test('lists every stage the server sends, in its order, each titled for the story', () => {
  // Arrange
  streamOnHead({ stages: makeStages() })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(storyTitles()).toEqual(['Issue', 'Branch', 'Changes', 'Pull request', 'Announce'])
})

test('lists only the stages the server sends', () => {
  // Arrange
  streamOnHead({ stages: makeStages().filter((stage) => stage.step !== 'review') })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(storyTitles()).toEqual(['Issue', 'Branch', 'Changes', 'Announce'])
})

// How a stage the server read is drawn: done and failed as they are, and one
// under way or not begun as the stage the work is at when it is the first not
// done, and still to come after it. The snapshot's own branch and pull request
// say the opposite in each case, so the story is seen to read the stage.
test.each<{ state: Stage['state']; drawn: string; mark: MarkState }>([
  { state: 'done', drawn: 'done', mark: 'done' },
  { state: 'failed', drawn: 'failed', mark: 'failed' },
  { state: 'in_flight', drawn: 'active', mark: 'in-flight' },
  { state: 'not_started', drawn: 'active', mark: 'in-flight' },
])('draws a review stage the server reads $state as $drawn', ({ state, drawn, mark }) => {
  // Arrange
  const ci: Ci = {
    state: state === 'done' ? 'failed' : 'passed',
    total: 1,
    done: 1,
    failed: 0,
    checks: [],
  }
  streamOnHead({
    review: { found: true, announced: false, pull: openPull, ci },
    stages: makeStages({ issue: 'done', branch: 'done', commits: 'done', review: state }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(storyStage('Pull request')).toEqual({ mark: drawnMark(mark), state: drawn })
})

test('draws the stages after the first not done as still to come', () => {
  // Arrange
  streamOnHead({ stages: makeStages({ issue: 'done', commits: 'done' }) })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect([storyStage('Branch'), storyStage('Changes'), storyStage('Announce')]).toEqual([
    { mark: drawnMark('in-flight'), state: 'active' },
    { mark: drawnMark('done'), state: 'done' },
    { mark: drawnMark('not-started'), state: 'upcoming' },
  ])
})

test('says the pull request was announced when the server reads the stage done', () => {
  // Arrange
  streamOnHead({ stages: makeStages({ announce: 'done' }) })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByRole('button', { name: /^Announce/ }).textContent).toContain('Announced')
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

// A branch name is what a reader would type into a terminal, so the story sets
// it in the code face wherever it names one — the Branch stage's detail and
// the note over an issue in flight elsewhere — and leaves the words around it
// in the body face.
test.each([
  { where: 'the stage of the checked-out branch', current: true },
  { where: 'the stage of a branch elsewhere', current: false },
])('sets the branch name in the code face in $where', ({ current }) => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [{ name: 'fix/PROJ-1', issue_key: 'PROJ-1', current }],
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  const stage = screen.getByRole('button', { name: /^Branch/ })
  expect(within(stage).getByText('fix/PROJ-1').tagName).toBe('CODE')
})

test('sets the branch name in the code face in the note over an issue in flight elsewhere', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [{ name: 'feat/PROJ-2-metrics', issue_key: 'PROJ-2', current: false }],
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-2" />)

  // Assert
  const note = screen.getByText(/^In progress on/)
  expect(within(note).getByText('feat/PROJ-2-metrics').tagName).toBe('CODE')
})

test.each([
  [1, '1 file to commit'],
  [3, '3 files to commit'],
])('counts %i changed files in words', (files, said) => {
  // Arrange
  const changed: Change = {
    path: 'a.go',
    kind: 'modified',
    staged: false,
    has_unstaged: true,
    conflicted: false,
  }
  streamOnHead({ changes: { changes: Array.from({ length: files }, () => changed) } })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByRole('button', { name: /^Changes/ }).textContent).toContain(said)
})
