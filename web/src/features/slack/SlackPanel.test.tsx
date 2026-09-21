import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { announce } from './announceApi.ts'
import { SlackPanel } from './SlackPanel.tsx'

vi.mock('./announceApi.ts', () => ({
  previewAnnouncement: vi.fn(() =>
    Promise.resolve({ text: 'octocat announced the pull request', channel: '#dev' }),
  ),
  announce: vi.fn(() => Promise.resolve()),
}))
const mockAnnounce = vi.mocked(announce)

// withPullRequest is a snapshot with Slack configured and a pull request to
// announce.
function withPullRequest() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      slack: { channel: '#dev', channels: ['#dev', '#releases'], author: 'ana.lopez' },
      review: {
        found: true,
        pull: {
          number: 42,
          url: 'https://x/42',
          title: 'redact',
          draft: false,
          approvals: 0,
          changes_requested: false,
          mergeable: 'clean',
        },
      },
    }),
  })
}

test('shows where and as whom a post would go', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      slack: { channel: '#dev', channels: ['#dev', '#releases'], author: 'ana.lopez' },
    }),
  })

  // Act
  render(<SlackPanel />)

  // Assert
  // #dev is both the default channel and a listed alternate, so it appears more
  // than once; the author and the second alternate are unique.
  expect(screen.getAllByText('#dev').length).toBeGreaterThan(0)
  expect(screen.getByText('ana.lopez')).toBeTruthy()
  expect(screen.getByText('#releases')).toBeTruthy()
})

test('names the webhook when there is no resolved author', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ slack: { channel: '#ops', channels: [], author: '' } }),
  })

  // Act
  render(<SlackPanel />)

  // Assert
  expect(screen.getByText(/the webhook/i)).toBeTruthy()
})

test('says when Slack is not configured', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ slack: { channel: '', channels: [], author: '' } }),
  })

  // Act
  render(<SlackPanel />)

  // Assert
  expect(screen.getByText(/not configured/i)).toBeTruthy()
})

test('offers to announce when a pull request exists', () => {
  // Arrange
  withPullRequest()

  // Act
  render(<SlackPanel />)

  // Assert
  expect(screen.getByRole('button', { name: /announce to slack/i })).toBeTruthy()
})

test('says there is nothing to announce without a pull request', () => {
  // Arrange
  // makeSnapshot's review has no pull request found.
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ slack: { channel: '#dev', channels: [], author: 'ana.lopez' } }),
  })

  // Act
  render(<SlackPanel />)

  // Assert
  expect(screen.getByText(/nothing to announce/i)).toBeTruthy()
  expect(screen.queryByRole('button', { name: /announce to slack/i })).toBeNull()
})

test('previews the message, then posts it on confirm', async () => {
  // Arrange
  const user = userEvent.setup()
  withPullRequest()
  render(<SlackPanel />)

  // Act: open the preview, then confirm
  await user.click(screen.getByRole('button', { name: /announce to slack/i }))
  await screen.findByText('octocat announced the pull request')
  await user.click(screen.getByRole('button', { name: /post to slack/i }))

  // Assert
  expect(mockAnnounce).toHaveBeenCalledWith('#dev')
  expect(await screen.findByText(/announced/i)).toBeTruthy()
})

test('does not post if the preview is canceled', async () => {
  // Arrange
  const user = userEvent.setup()
  withPullRequest()
  render(<SlackPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /announce to slack/i }))
  await screen.findByText('octocat announced the pull request')
  await user.click(screen.getByRole('button', { name: /cancel/i }))

  // Assert
  expect(mockAnnounce).not.toHaveBeenCalled()
})

test('shows the reason when the post is refused', async () => {
  // Arrange
  mockAnnounce.mockRejectedValueOnce({
    code: 'unprocessable',
    message: 'the announcement could not be posted',
  })
  const user = userEvent.setup()
  withPullRequest()
  render(<SlackPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /announce to slack/i }))
  await screen.findByText('octocat announced the pull request')
  await user.click(screen.getByRole('button', { name: /post to slack/i }))

  // Assert
  expect(await screen.findByText(/could not be posted/i)).toBeTruthy()
})

test('prompts to connect before any snapshot arrives', () => {
  // Act
  render(<SlackPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})
