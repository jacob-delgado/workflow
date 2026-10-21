import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PullRequest, QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { MessagingPanel } from './MessagingPanel.tsx'

// announcedPull streams a configured Slack with pull request #42, announced or
// not at the moment it is at now, open unless it was closed or merged, and
// the announcement held for its CI, if any.
function announcedPull(
  announced: boolean,
  state: PullRequest['state'] = 'open',
  held?: QueuedAnnouncement,
) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      messaging: {
        kind: 'slack',
        service: 'Slack',
        configured: true,
        channel: '#dev',
        channels: [],
        author: 'ana.lopez',
      },
      review: {
        found: true,
        announced,
        pull: {
          number: 42,
          url: 'https://x/42',
          title: 'redact',
          state,
          draft: false,
          approvals: 0,
          changes_requested: false,
          mergeable: 'clean',
        },
      },
      queued_announcement: held,
    }),
  })
}

test('says an announcement already made, from anywhere, and offers no second one', () => {
  // Arrange
  announcedPull(true)

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText('Announced #42 as ready for review.')).toBeTruthy()
  expect(screen.queryByRole('button', { name: /^Announce to/ })).toBeNull()
})

test.each([
  { state: 'open', said: 'Announced #42 as ready for review.' },
  { state: 'merged', said: 'Announced the merge of #42.' },
] as const)(
  'says an announcement already made by the moment it was made at: $state',
  ({ state, said }) => {
    // Arrange
    announcedPull(true, state)

    // Act
    render(<MessagingPanel />)

    // Assert
    expect(screen.getByText(said)).toBeTruthy()
  },
)

test('says a held announcement went once, not twice', () => {
  // Arrange
  announcedPull(true, 'open', { state: 'announced', channel: '#dev', pull: 42 })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText('Announced to #dev once CI passed.')).toBeTruthy()
  expect(screen.queryByText(/^Announced #42/)).toBeNull()
})

test('says an announcement made here once, not twice', async () => {
  // Arrange
  fakeApi({
    '/api/announcement': { text: 'Please review #42', channel: '#dev' },
    '/api/announce': { text: 'Please review #42', channel: '#dev' },
  })
  announcedPull(false)
  render(<MessagingPanel />)
  await userEvent.click(screen.getByRole('button', { name: 'Announce to Slack' }))
  await userEvent.click(await screen.findByRole('button', { name: 'Announce now' }))
  await screen.findByText('Announced to #dev.')

  // Act
  act(() => {
    announcedPull(true)
  })

  // Assert
  expect(screen.getByText('Announced to #dev.')).toBeTruthy()
  expect(screen.queryByText(/^Announced #42/)).toBeNull()
})

test('offers the announcement while it has not been made', () => {
  // Arrange
  announcedPull(false)

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Announce to Slack' })).toBeTruthy()
})

test('offers nothing to announce for a pull request closed without merging', () => {
  // Arrange
  // A closed pull request is none the loop follows, so the server never reads
  // it as announced, and there is nothing to announce until one is open again.
  announcedPull(false, 'closed')

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(
    screen.getByText('Open a pull request first — there is nothing to announce yet.'),
  ).toBeTruthy()
  expect(screen.queryByRole('button', { name: /^Announce to/ })).toBeNull()
})

test('never says a pull request closed without merging was announced', () => {
  // Arrange
  // The page reads the pull request's state itself, so even a frame that read
  // a closed one as announced would not have it said announced.
  announcedPull(true, 'closed')

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.queryByText(/^Announced #42/)).toBeNull()
  expect(
    screen.getByText('Open a pull request first — there is nothing to announce yet.'),
  ).toBeTruthy()
})
