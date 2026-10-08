import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PullRequest, QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { MessagingPanel } from './MessagingPanel.tsx'

// Editing the announcement before it goes, and holding it until the pull
// request's CI passes, as the terminal's e and w in the preview do; and saying
// of one posted that the store could not remember that it may be offered
// again, as the terminal and workflow announce do.

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

// notRemembered is the server's warning on a post the store could not remember.
const notRemembered = 'Posted, but not remembered: it may be offered again.'

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

// serverAnswering previews the composed announcement — one that can wait for
// CI when canWait says so — and answers a post as given, keeping each write's
// method, path and body.
function serverAnswering({
  canWait = false,
  post = Response.json({ text: composed, channel: '#dev' }),
} = {}) {
  const sent: { to: string; body: unknown }[] = []
  fakeApi({
    '/api/announcement': {
      text: composed,
      channel: '#dev',
      ...(canWait ? { can_wait_for_ci: true } : {}),
    },
    '/api/announce': async (_: URL, request: Request) => {
      sent.push({ to: 'POST /api/announce', body: await request.json() })

      return post.clone()
    },
    '/api/announce/queued': (_: URL, request: Request) => {
      sent.push({ to: `${request.method} /api/announce/queued`, body: null })

      return new Response(null, { status: 204 })
    },
  })

  return sent
}

// previews opens the announcement's preview.
async function previews() {
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))
  await screen.findByRole('group', { name: 'Announcement preview' })
}

test('an edited announcement is posted as edited, beside the text it began from', async () => {
  // Arrange
  const sent = serverAnswering({
    post: Response.json({ text: 'Please review #42', channel: '#dev' }),
  })
  withPull()
  render(<MessagingPanel />)
  await previews()
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
  const box = screen.getByRole('textbox', { name: 'Announcement text' })
  await userEvent.clear(box)
  await userEvent.type(box, 'Please review #42')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Announce now' }))

  // Assert
  await screen.findByText('Announced to #dev.')
  expect(sent).toEqual([
    {
      to: 'POST /api/announce',
      body: { channel: '#dev', text: composed, edited_text: 'Please review #42' },
    },
  ])
})

test('an announcement that can wait is held until CI passes', async () => {
  // Arrange
  const sent = serverAnswering({
    canWait: true,
    post: Response.json({ state: 'waiting', channel: '#dev', pull: 42 }, { status: 202 }),
  })
  withPull()
  render(<MessagingPanel />)
  await previews()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Announce when CI passes' }))

  // Assert
  await screen.findByText('Will announce to #dev once CI passes.')
  expect(sent).toEqual([
    { to: 'POST /api/announce', body: { channel: '#dev', text: composed, when: 'ci_passes' } },
  ])
})

test('an announcement that cannot wait offers no wait', async () => {
  // Arrange
  serverAnswering()
  withPull()
  render(<MessagingPanel />)

  // Act
  await previews()

  // Assert
  expect(screen.queryByRole('button', { name: 'Announce when CI passes' })).toBeNull()
})

test('a held announcement says it waits, and stops waiting on asking', async () => {
  // Arrange
  const sent = serverAnswering()
  withPull({ state: 'waiting', channel: '#dev', pull: 42 })
  render(<MessagingPanel />)

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Stop waiting' }))

  // Assert
  expect(screen.getByText('Waiting for CI on #42 to pass, then announcing to #dev.')).toBeTruthy()
  await waitFor(() => {
    expect(sent).toEqual([{ to: 'DELETE /api/announce/queued', body: null }])
  })
  expect(await screen.findByText('Stopped waiting; nothing was announced.')).toBeTruthy()
})

test('a refused Stop waiting says why, with focus still on it', async () => {
  // Arrange
  fakeApi({
    '/api/announce/queued': () =>
      Response.json({ code: 'unprocessable', detail: 'nothing is waiting' }, { status: 422 }),
  })
  withPull({ state: 'waiting', channel: '#dev', pull: 42 })
  render(<MessagingPanel />)
  const stop = screen.getByRole('button', { name: 'Stop waiting' })

  // Act
  await userEvent.click(stop)

  // Assert
  expect(await screen.findByRole('alert')).toBeTruthy()
  expect(document.activeElement).toBe(stop)
})

test('a held announcement that went says so', () => {
  // Arrange
  serverAnswering()
  withPull({ state: 'announced', channel: '#dev', pull: 42 })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText('Announced to #dev once CI passed.')).toBeTruthy()
})

test('a held announcement that was dropped says why, as a failure', () => {
  // Arrange
  serverAnswering()
  withPull({ state: 'dropped', channel: '#dev', pull: 42, reason: 'CI failed at 10:00' })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByRole('alert').textContent).toBe('Not announced: CI failed at 10:00.')
})

test('a held announcement going out says so', () => {
  // Arrange
  serverAnswering()
  withPull({ state: 'announcing', channel: '', pull: 42 })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText('Announcing to Slack…')).toBeTruthy()
})

test('a post the server held instead is not taken for posted', async () => {
  // Arrange
  serverAnswering({
    post: Response.json({ state: 'waiting', channel: '#dev', pull: 42 }, { status: 202 }),
  })
  withPull()
  render(<MessagingPanel />)
  await previews()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Announce now' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'Nothing was announced. Try again, or run workflow announce from a terminal.',
  )
})

test('an announcement posted but not remembered says it may be offered again', async () => {
  // Arrange
  serverAnswering({
    post: Response.json({ text: composed, channel: '#dev', warning: notRemembered }),
  })
  withPull()
  render(<MessagingPanel />)
  await previews()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Announce now' }))

  // Assert
  expect(await screen.findByText(`Announced to #dev. ${notRemembered}`)).toBeTruthy()
})

test('an announcement asked to wait that went at once, not remembered, says so', async () => {
  // Arrange
  serverAnswering({
    canWait: true,
    post: Response.json({ text: composed, channel: '#dev', warning: notRemembered }),
  })
  withPull()
  render(<MessagingPanel />)
  await previews()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Announce when CI passes' }))

  // Assert
  expect(await screen.findByText(`Announced to #dev. ${notRemembered}`)).toBeTruthy()
})

test('a held announcement that went, not remembered, says it may be offered again', () => {
  // Arrange
  serverAnswering()
  withPull({ state: 'announced', channel: '#dev', pull: 42, warning: notRemembered })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText(`Announced to #dev once CI passed. ${notRemembered}`)).toBeTruthy()
})
