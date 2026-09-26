import { render, screen } from '@testing-library/react'
import type { PullRequest } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

// A pull request that has merged, as the server sends it: no CI, since a merged
// pull request has none live to ask about, and the reviews and mergeability the
// forge stops reporting once it is no longer open.
const merged: PullRequest = {
  number: 128,
  url: 'https://forge.example.com/pull/128',
  title: 'Redact tokens in the request log',
  state: 'merged',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'unknown',
}

function showReview(pull: PullRequest) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: true, pull } }),
  })
}

test('says a merged pull request merged, not that it is ready for review', () => {
  // Arrange
  showReview(merged)

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByText('Merged')).toBeTruthy()
  expect(screen.queryByText('Ready for review')).toBeNull()
})

test('shows a merged pull request without the review it no longer waits on', () => {
  // Arrange
  showReview(merged)

  // Act
  render(<ReviewPanel />)

  // Assert
  for (const row of ['Mergeable', 'Approvals', 'Changes requested']) {
    expect(screen.queryByText(row)).toBeNull()
  }
  expect(screen.queryByRole('heading', { name: /CI checks/ })).toBeNull()
  expect(screen.getByRole('link', { name: merged.title })).toBeTruthy()
})

// What the State row says for the other states a pull request can be in on the
// wire: an open one is a draft or ready for review, and one closed without
// merging says so.
const stateWords = [
  { state: 'open', draft: false, says: 'Ready for review' },
  { state: 'open', draft: true, says: 'Draft' },
  { state: 'closed', draft: false, says: 'Closed' },
] as const

test.each(stateWords)(
  'says $says for a pull request $state (draft: $draft)',
  ({ state, draft, says }) => {
    // Arrange
    showReview({ ...merged, state, draft })

    // Act
    render(<ReviewPanel />)

    // Assert
    expect(screen.getByText(says)).toBeTruthy()
  },
)
