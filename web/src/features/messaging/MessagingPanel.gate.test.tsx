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
      review: { found: true, announced: 'none', pull },
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

test('a preview of a merge a check failed on says to re-run the checks, offering no wait', async () => {
  // Arrange
  fakeApi({
    '/api/announcement': () =>
      Response.json({ text: composed, channel: '#dev', ci: 'failed', can_wait_for_ci: false }),
  })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: { found: true, announced: 'none', pull: { ...pull, state: 'merged' } },
    }),
  })
  render(<MessagingPanel />)

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))

  // Assert
  expect(
    await screen.findByText(
      'A check failed on #42, so its merge is not announced. ' +
        'Re-run the failed checks on the forge, then announce it once they pass.',
    ),
  ).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Announce when CI passes' })).toBeNull()
})

// failingBeforePost previews with CI running, then has a check fail before
// the post, which the server refuses for it, while it holds one until CI
// passes; it returns the body of each announcement sent.
function failingBeforePost(): unknown[] {
  let previews = 0
  const sent: unknown[] = []
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
    '/api/announce': async (_at: URL, asked: Request) => {
      const body = (await asked.json()) as { when?: string }
      sent.push(body)

      return body.when === 'ci_passes'
        ? Response.json({ id: 1, state: 'waiting', channel: '#dev', pull: 42 }, { status: 202 })
        : Response.json(
            {
              type: 'https://jacob-delgado.github.io/workflow/docs/errors/#ci-failed',
              title: 'CI failed',
              status: 409,
              detail: 'a check failed on #42, so nothing was announced',
              code: 'ci_failed',
            },
            { status: 409, headers: { 'Content-Type': 'application/problem+json' } },
          )
    },
  })

  return sent
}

test('a post refused because a check failed offers to wait', async () => {
  // Arrange
  failingBeforePost()
  withPull()
  render(<MessagingPanel />)
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))

  // Act
  await userEvent.click(await screen.findByRole('button', { name: 'Announce now' }))

  // Assert
  expect(await screen.findByRole('button', { name: 'Announce when CI passes' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Announce now' })).toBeNull()
})

test('a post refused because a check failed keeps the edit for the wait', async () => {
  // Arrange
  const sent = failingBeforePost()
  withPull()
  render(<MessagingPanel />)
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))
  await userEvent.click(await screen.findByRole('button', { name: 'Edit' }))
  const box = screen.getByRole('textbox', { name: 'Announcement text' })
  await userEvent.clear(box)
  await userEvent.type(box, 'Please review #42')
  await userEvent.click(screen.getByRole('button', { name: 'Announce now' }))

  // Act
  await userEvent.click(await screen.findByRole('button', { name: 'Announce when CI passes' }))

  // Assert
  await screen.findByText('Will announce to #dev once CI passes.')
  expect(sent.at(-1)).toEqual({
    channel: '#dev',
    text: composed,
    edited_text: 'Please review #42',
    when: 'ci_passes',
  })
})

test('a held announcement waiting on a fix says so', () => {
  // Arrange
  previewing('failed')
  withPull({ id: 1, state: 'waiting', channel: '#dev', pull: 42, waiting_on: 'fix', ci: 'failed' })

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
  withPull({ id: 1, state: 'waiting', channel: '#dev', pull: 42, waiting_on: 'checks', ci: 'none' })

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

test('an armed announcement says it was armed as the pull request opened', () => {
  // Arrange
  previewing('running')
  withPull({
    id: 1,
    state: 'waiting',
    channel: '#dev',
    pull: 42,
    waiting_on: 'ci',
    ci: 'running',
    automatic: true,
  })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText('Armed when #42 opened.', { exact: false })).toBeTruthy()
})
