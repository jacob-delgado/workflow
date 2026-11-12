import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PullRequest, QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { MessagingPanel } from './MessagingPanel.tsx'

// The announce controls follow the held announcement the stream shows, frame
// by frame: what the section offers comes from the frame, never from what was
// pressed on this page before it.

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

// heldAs is the announcement of #42 to #dev held as the given id, standing as
// state.
function heldAs(id: number, state: QueuedAnnouncement['state']): QueuedAnnouncement {
  return {
    id,
    state,
    channel: '#dev',
    pull: 42,
    ...(state === 'waiting' ? { waiting_on: 'ci', ci: 'running' } : {}),
  }
}

// frame pushes the stream's next frame: pull request 42 open, not announced,
// and the held announcement given, if any.
function frame(held?: QueuedAnnouncement) {
  pushFrame(
    makeSnapshot({
      review: { found: true, announced: 'none', pull },
      queued_announcement: held,
    }),
  )
}

// serverHolding previews an announcement that can wait for CI, holds it as
// asked, and stops it as asked.
function serverHolding() {
  fakeApi({
    '/api/announcement': { text: composed, channel: '#dev', can_wait_for_ci: true },
    '/api/announce': () => Response.json(heldAs(1, 'waiting'), { status: 202 }),
    '/api/announce/queued': () => new Response(null, { status: 204 }),
  })
}

// heldFromThePreview opens the preview and holds the announcement until CI
// passes, the frame that follows showing it waiting.
async function heldFromThePreview() {
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))
  await screen.findByRole('group', { name: 'Announcement preview' })
  await userEvent.click(screen.getByRole('button', { name: 'Announce when CI passes' }))
  await screen.findByText('Will announce to #dev once CI passes.')
  act(() => {
    frame(heldAs(1, 'waiting'))
  })
}

test('once one is held here, Announce now instead is offered', async () => {
  // Arrange
  serverHolding()
  frame()
  render(<MessagingPanel />)

  // Act
  await heldFromThePreview()

  // Assert
  expect(screen.getByRole('button', { name: 'Announce now instead' })).toBeTruthy()
})

test('once one held here is stopped, Announce to is offered again', async () => {
  // Arrange
  serverHolding()
  frame()
  render(<MessagingPanel />)
  await heldFromThePreview()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Stop waiting' }))
  await screen.findByText('Stopped waiting; nothing was announced.')
  act(() => {
    frame(heldAs(1, 'stopped'))
  })

  // Assert
  expect(screen.getByRole('button', { name: 'Announce to Slack' })).toBeTruthy()
})

test('a hold armed afresh before the stop shows offers Stop waiting again', async () => {
  // Arrange
  serverHolding()
  frame(heldAs(1, 'waiting'))
  render(<MessagingPanel />)
  await userEvent.click(screen.getByRole('button', { name: 'Stop waiting' }))
  await screen.findByText('Stopped waiting; nothing was announced.')

  // Act
  act(() => {
    frame(heldAs(2, 'waiting'))
  })

  // Assert
  const stop = screen.getByRole('button', { name: 'Stop waiting' })
  expect(stop.hasAttribute('aria-disabled')).toBe(false)
})

test.each([
  {
    waiting: 'for the forge to list it',
    held: { waiting_on: 'listing' },
    says: 'Waiting for the forge to list #42, then announcing to #dev once CI passes.',
  },
  {
    waiting: 'past a read that failed',
    held: { waiting_on: 'unread', reason: 'the service could not be reached' },
    says:
      'Not announced yet: the service could not be reached. Trying again, then announcing to #dev ' +
      'once CI passes.',
  },
] satisfies { waiting: string; held: Partial<QueuedAnnouncement>; says: string }[])(
  'a held announcement waiting $waiting says so',
  ({ held, says }) => {
    // Arrange
    serverHolding()

    // Act
    frame({ ...heldAs(1, 'waiting'), ...held })
    render(<MessagingPanel />)

    // Assert
    expect(screen.getByText(says)).toBeTruthy()
  },
)

test('a held announcement another session posted first says it was not posted here', () => {
  // Arrange
  serverHolding()

  // Act
  frame({ ...heldAs(1, 'elsewhere'), reason: '#42 was already announced at this moment' })
  render(<MessagingPanel />)

  // Assert
  expect(
    screen.getByText('Not posted here: #42 was already announced at this moment.'),
  ).toBeTruthy()
})

test('a held announcement stopped says nothing of its own', () => {
  // Arrange
  serverHolding()

  // Act
  frame(heldAs(1, 'stopped'))
  render(<MessagingPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Stop waiting' })).toBeNull()
  expect(screen.queryByText('Not announced', { exact: false })).toBeNull()
})

test('holding one in place of another pull request’s says which it replaced', async () => {
  // Arrange
  fakeApi({
    '/api/announcement': { text: composed, channel: '#dev', can_wait_for_ci: true },
    '/api/announce': () =>
      Response.json({ ...heldAs(2, 'waiting'), replaced_pull: 41 }, { status: 202 }),
  })
  frame()
  render(<MessagingPanel />)
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))
  await screen.findByRole('group', { name: 'Announcement preview' })

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Announce when CI passes' }))

  // Assert
  expect(
    await screen.findByText(
      'Will announce to #dev once CI passes. It replaced the announcement waiting for #41.',
    ),
  ).toBeTruthy()
})
