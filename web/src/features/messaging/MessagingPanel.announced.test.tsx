import { render, screen } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { MessagingPanel } from './MessagingPanel.tsx'

// announcedPull streams a configured Slack with pull request #42, announced or
// not at the moment it is at now, open unless it was closed.
function announcedPull(announced: boolean, state: 'open' | 'closed' = 'open') {
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
    }),
  })
}

test('says an announcement already made, from anywhere, and offers no second one', () => {
  // Arrange
  announcedPull(true)

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText('Announced #42 at this point already.')).toBeTruthy()
  expect(screen.queryByRole('button', { name: /^Announce to/ })).toBeNull()
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
