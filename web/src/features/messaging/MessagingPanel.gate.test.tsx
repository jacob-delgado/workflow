import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { CiState, PullRequest, QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { MessagingPanel } from './MessagingPanel.tsx'

// Nothing is announced while a check has failed on the pull request: the
// preview offers only to wait for a fix, a post the server refuses for it
// offers the wait, and a held announcement says what it waits on.

const pull: PullRequest = {
  number: 42,
  url: 'https://forge.example.com/pull/42',
  title: 'Redact tokens in the request log',
  state: 'open',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'clean',
}

const composed = 'octocat opened a pull request: Redact tokens\nhttps://forge.example.com/pull/42'

// withPull is the panel's snapshot: pull request 42 open, and the held
// announcement given, if any.
function withPull(held?: QueuedAnnouncement) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: { found: true, announced: false, pull },
      queued_announcement: held,
    }),
  })
}

// previewing answers the preview with CI standing as ci, and a post with
// post.
function previewing(ci: CiState, post: () => Response = () => Response.json({})) {
  fakeApi({
    '/api/announcement': () =>
      Response.json({ text: composed, channel: '#dev', ci, can_wait_for_ci: ci !== 'passed' }),
    '/api/announce': post,
  })
}

test('a preview of a pull request a check failed on offers only to announce when CI passes', async () => {
  // Arrange
  previewing('failed')
  withPull()
  render(<MessagingPanel />)

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))

  // Assert
  expect(await screen.findByRole('button', { name: 'Announce when CI passes' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Announce now' })).toBeNull()
  expect(
    screen.getByText(
      'A check failed on #42, so it is not announced now. Announce when CI passes waits for a fix.',
    ),
  ).toBeTruthy()
})

test('a post refused because a check failed offers to wait', async () => {
  // Arrange
  let previews = 0
  fakeApi({
    '/api/announcement': () => {
      previews += 1

      return Response.json({
        text: composed,
        channel: '#dev',
        ci: previews === 1 ? 'running' : 'failed',
        can_wait_for_ci: true,
      })
    },
    '/api/announce': () =>
      Response.json(
        {
          type: 'https://jacob-delgado.github.io/workflow/docs/errors/#ci-failed',
          title: 'CI failed',
          status: 409,
          detail: 'a check failed on #42, so nothing was announced',
          code: 'ci_failed',
        },
        { status: 409, headers: { 'Content-Type': 'application/problem+json' } },
      ),
  })
  withPull()
  render(<MessagingPanel />)
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))

  // Act
  await userEvent.click(await screen.findByRole('button', { name: 'Announce now' }))

  // Assert
  expect(await screen.findByRole('button', { name: 'Announce when CI passes' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Announce now' })).toBeNull()
})

test('a held announcement waiting on a fix says so', () => {
  // Arrange
  previewing('failed')
  withPull({ state: 'waiting', channel: '#dev', pull: 42, ci: 'failed' })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(
    screen.getByText(
      'A check failed on #42; waiting on a fix, then announcing to #dev once CI passes.',
    ),
  ).toBeTruthy()
})

test('one waiting for checks says a repository with no CI is announced by hand', () => {
  // Arrange
  previewing('none')
  withPull({ state: 'waiting', channel: '#dev', pull: 42, ci: 'none' })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(
    screen.getByText(
      'Waiting for checks to be reported on #42, then announcing to #dev once CI passes. ' +
        'A repository with no CI is announced by hand.',
    ),
  ).toBeTruthy()
})
