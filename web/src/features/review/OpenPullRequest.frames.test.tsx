import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PullRequest, PullRequestDraft } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { MessagingPanel } from '@/features/messaging/MessagingPanel.tsx'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

// Opening a pull request beside the panes it changes: the server wakes the
// stream as it answers, and the page hands the answer on only once the frame
// it woke has landed, so the notice never stands beside an Announce section
// still asking for a pull request to be opened first.

const draft: PullRequestDraft = {
  title: 'fix: redact tokens',
  body: '- fix: redact tokens\n',
  base: 'main',
  head: 'fix/PROJ-1',
  draft: false,
  needs_push: false,
  reviewers: [],
  templates: [],
  template: '',
}

const opened: PullRequest = {
  number: 128,
  url: 'https://forge.example.com/pull/128',
  title: 'fix: redact tokens',
  state: 'open',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'clean',
}

test('Opened pull request #128 is said only once the page shows it open', async () => {
  // Arrange
  const user = userEvent.setup()
  const armed = { state: 'waiting', channel: '#dev', pull: 128, automatic: true } as const
  fakeApi(
    {
      '/api/pull-request/draft': draft,
      '/api/pull-request': { pull: opened, follow_ups: [], announcement: armed },
    },
    { frames: 'held' },
  )
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(
    <>
      <ReviewPanel />
      <MessagingPanel />
    </>,
  )
  await user.click(screen.getByRole('button', { name: 'Open a pull request' }))
  const form = await screen.findByRole('form', { name: 'Open a pull request' })

  // Act: open it, with the frame the open woke not yet landed
  await user.click(within(form).getByRole('button', { name: 'Open pull request' }))
  await new Promise((resolve) => setTimeout(resolve, 0))

  // Assert: nothing says it opened while Announce still asks for one
  expect(screen.queryByText('Opened pull request #128.')).toBeNull()
  expect(screen.getByText(/Open a pull request first/)).toBeTruthy()

  // Act: the frame lands
  act(() => {
    pushFrame(
      makeSnapshot({
        review: { found: true, announced: false, pull: opened },
        queued_announcement: armed,
      }),
    )
  })

  // Assert: the notice stands beside a page that shows the pull request open
  await screen.findByText('Opened pull request #128.')
  expect(screen.queryByText(/Open a pull request first/)).toBeNull()
})
