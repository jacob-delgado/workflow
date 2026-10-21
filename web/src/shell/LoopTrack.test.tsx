import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Change } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot, makeStages } from '@/test/fixtures.ts'
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

// stagesSaid is each stage on the track as a screen reader reads it.
function stagesSaid(): (string | null)[] {
  return within(track())
    .getAllByRole('listitem')
    .map((item) => item.textContent)
}

test("the loop track agrees with a write's notice", async () => {
  // Arrange
  const user = userEvent.setup()
  const committed = makeBranch({
    head: 'def5678',
    commits: [{ hash: 'def5678', subject: 'fix: redact tokens', unpushed: true }],
  })
  const staged: Change = {
    path: 'a.go',
    kind: 'modified',
    staged: true,
    has_unstaged: false,
    conflicted: false,
  }
  fakeApi({ '/api/commit': committed }, { frames: 'held' })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      changes: { changes: [staged] },
      stages: makeStages({ issue: 'done', branch: 'done' }),
    }),
  })
  render(
    <>
      <LoopTrack />
      <BranchPanel />
    </>,
  )
  await user.type(screen.getByRole('textbox', { name: /subject/i }), 'redact tokens')

  // Act: commit, with the frame the commit woke not yet landed
  await user.click(screen.getByRole('button', { name: 'Commit staged changes' }))
  await new Promise((resolve) => setTimeout(resolve, 0))

  // Assert: nothing says it committed while the track has no commits
  expect(screen.queryByText(/^Committed /)).toBeNull()
  expect(stagesSaid()).toContain('Commits: not started')

  // Act: the frame lands
  act(() => {
    pushFrame(
      makeSnapshot({
        branch: committed,
        stages: makeStages({ issue: 'done', branch: 'done', commits: 'done' }),
      }),
    )
  })

  // Assert: the notice stands beside a track that has the commit done
  await screen.findByText('Committed def5678 fix: redact tokens.')
  expect(stagesSaid()).toContain('Commits: done')
})
