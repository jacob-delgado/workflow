import { render, screen } from '@testing-library/react'
import type { Problem, Review } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { ReviewPanel } from './ReviewPanel.tsx'

// unreachable is the problem the server sends for a forge it could not reach.
const unreachable: Problem = {
  type: 'https://jacob-delgado.github.io/workflow/docs/errors/#unreachable',
  title: 'Upstream unreachable',
  status: 502,
  detail: 'the service could not be reached; check the network, then try again',
  code: 'unreachable',
}

const openPull: NonNullable<Review['pull']> = {
  number: 128,
  url: 'https://forge.example.com/pull/128',
  title: 'Redact tokens in the request log',
  state: 'open',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean',
}

// streamReview puts a frame on screen with the review, and the review's
// problem when there is one.
function streamReview(review: Review, problem?: Problem) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review, problems: problem ? { review: problem } : undefined }),
  })
}

test('says the forge could not be read, rather than offer to open a pull request', () => {
  // Arrange
  streamReview({ found: false, announced: false }, unreachable)

  // Act
  render(<ReviewPanel />)

  // Assert
  const alert = screen.getByRole('alert')
  expect(alert.textContent).toBe(
    'The pull request could not be read: the service could not be reached; check the network, then try again',
  )
  expect(markShape(alert.parentElement ?? alert)).toBe(drawnMark('failed'))
  expect(screen.queryByRole('button', { name: 'Open a pull request' })).toBeNull()
})

test('keeps the pull request last read beside why it could not be read again', () => {
  // Arrange
  streamReview({ found: true, announced: false, pull: openPull }, unreachable)

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByRole('heading', { name: /Redact tokens in the request log/ })).toBeTruthy()
  expect(screen.getByRole('alert').textContent).toBe(
    `The pull request could not be read again; shown as last read: ${unreachable.detail}`,
  )
})

test('says why the CI could not be read when the review carries no CI', () => {
  // Arrange
  streamReview({ found: true, announced: false, pull: openPull, ci: null, ci_error: unreachable })

  // Act
  render(<ReviewPanel />)

  // Assert
  const alert = screen.getByRole('alert')
  expect(alert.textContent).toBe(
    'CI could not be read: the service could not be reached; check the network, then try again',
  )
  expect(screen.queryByRole('heading', { name: /CI checks/ })).toBeNull()
})

test('says nothing failed when the forge answered no pull request', () => {
  // Arrange
  streamReview({ found: false, announced: false })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.queryByRole('alert')).toBeNull()
  expect(screen.getByRole('button', { name: 'Open a pull request' })).toBeTruthy()
})
