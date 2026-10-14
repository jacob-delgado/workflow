import { render, screen, within } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot, makeStages } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { LoopTrack } from './LoopTrack.tsx'

// The loop's track, under the header: the checked-out branch's five stages in
// order, each by its mark, its name and its state in words.

// track is the loop's track as a screen reader meets it.
function track(): HTMLElement {
  return screen.getByRole('list', { name: 'Where the checked-out branch is in the loop' })
}

test('the track names each stage and how far it has got, in the order of the loop', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      stages: makeStages({ issue: 'done', branch: 'done', commits: 'done', review: 'in_flight' }),
    }),
  })

  // Act
  render(<LoopTrack />)

  // Assert
  expect(
    within(track())
      .getAllByRole('listitem')
      .map((item) => item.textContent),
  ).toEqual([
    'Issue: done',
    'Branch: done',
    'Commits: done',
    'Review: in flight',
    'Slack: not started',
  ])
})

test('each stage is drawn by its state’s mark', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ stages: makeStages({ issue: 'done', branch: 'failed' }) }),
  })

  // Act
  render(<LoopTrack />)

  // Assert
  const [issue, branch, commits] = within(track()).getAllByRole('listitem')
  expect(issue && markShape(issue)).toBe(drawnMark('done'))
  expect(branch && markShape(branch)).toBe(drawnMark('failed'))
  expect(commits && markShape(commits)).toBe(drawnMark('not-started'))
})

test('a review in flight says how many of its checks have finished', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      stages: makeStages({ issue: 'done', branch: 'done', commits: 'done', review: 'in_flight' }),
      review: {
        found: true,
        announced: false,
        ci: { state: 'running', total: 7, done: 5, failed: 0, checks: [] },
      },
    }),
  })

  // Act
  render(<LoopTrack />)

  // Assert
  expect(
    within(track())
      .getByText(/^Review/)
      .closest('li')?.textContent,
  ).toBe('Review: in flight, 5 of 7 checks finished')
})

test('the track is drawn only once the stream has said where the branch is', () => {
  // Act
  render(<LoopTrack />)

  // Assert
  expect(screen.queryByRole('list')).toBeNull()
})
