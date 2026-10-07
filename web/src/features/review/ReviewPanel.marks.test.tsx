import { render, screen } from '@testing-library/react'
import type { PullRequest } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import type { MarkState } from '@/shell/StateMark.tsx'
import { ReviewPanel } from './ReviewPanel.tsx'

// Each state the Review section's rows say is drawn by its mark, before its
// word, as every state on the web is.

const open: PullRequest = {
  number: 128,
  url: 'https://forge.example.com/pull/128',
  title: 'Redact tokens in the request log',
  state: 'open',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean',
}

// showing puts the pull request given on the branch's review.
function showing(pull: PullRequest) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: true, announced: false, pull } }),
  })
}

test.each<{ pull: Partial<PullRequest>; word: string; mark: MarkState }>([
  { pull: { state: 'open' }, word: 'Ready for review', mark: 'in-flight' },
  { pull: { state: 'open', draft: true }, word: 'Draft', mark: 'in-flight' },
  { pull: { state: 'merged' }, word: 'Merged', mark: 'done' },
  { pull: { state: 'closed' }, word: 'Closed', mark: 'not-started' },
  { pull: { mergeable: 'clean' }, word: 'No conflicts', mark: 'done' },
  { pull: { mergeable: 'conflicts' }, word: 'Has conflicts', mark: 'failed' },
  { pull: { mergeable: 'unknown' }, word: 'Mergeability unknown', mark: 'not-started' },
  { pull: { changes_requested: true }, word: 'Yes', mark: 'failed' },
])('draws "$word" as the $mark mark, before the word', ({ pull, word, mark }) => {
  // Arrange
  showing({ ...open, ...pull })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(markShape(screen.getByText(word))).toBe(drawnMark(mark))
})
