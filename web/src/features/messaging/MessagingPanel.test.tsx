import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { announce, previewAnnouncement } from './announceApi.ts'
import { MessagingPanel } from './MessagingPanel.tsx'

vi.mock('./announceApi.ts', () => ({
  previewAnnouncement: vi.fn(() =>
    Promise.resolve({ text: 'octocat announced the pull request', channel: '#dev' }),
  ),
  announce: vi.fn(() => Promise.resolve()),
}))
const mockAnnounce = vi.mocked(announce)
const mockPreview = vi.mocked(previewAnnouncement)

// withPullRequest is a snapshot with a service configured and a pull request to
// announce; a case that turns on the channel wiring passes its own destination.
function withPullRequest(
  messaging: Snapshot['messaging'] = {
    service: 'Slack',
    configured: true,
    channel: '#dev',
    channels: ['#dev', '#releases'],
    author: 'ana.lopez',
  },
) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      messaging,
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

test('shows the service, and where and as whom a post would go', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      messaging: {
        service: 'Slack',
        configured: true,
        channel: '#dev',
        channels: ['#dev', '#releases'],
        author: 'ana.lopez',
      },
    }),
  })

  // Act
  render(<MessagingPanel />)

  // Assert
  // #dev is both the default channel and a listed alternate, so it appears more
  // than once; the service, the author and the second alternate are unique.
  expect(screen.getByText('Slack')).toBeTruthy()
  expect(screen.getAllByText('#dev').length).toBeGreaterThan(0)
  expect(screen.getByText('ana.lopez')).toBeTruthy()
  expect(screen.getByText('#releases')).toBeTruthy()
})

test('names the webhook when there is no resolved author', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      messaging: { service: 'Slack', configured: true, channel: '#ops', channels: [], author: '' },
    }),
  })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText(/the webhook/i)).toBeTruthy()
})

test('names the configured service when it is not set up', () => {
  // Arrange
  // A Teams config with no transport yet — the empty-state message must name the
  // chosen service, not a hardcoded "Slack".
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      messaging: { service: 'Teams', configured: false, channel: '', channels: [], author: '' },
    }),
  })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText(/Teams is not configured/i)).toBeTruthy()
})

test('shows the panel for a configured webhook that has no channel', () => {
  // Arrange
  // A Teams webhook is fully configured yet carries no channel of its own — the
  // panel must not mistake the absent channel for "not configured" and hide the
  // Announce controls.
  withPullRequest({ service: 'Teams', configured: true, channel: '', channels: [], author: '' })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.queryByText(/is not configured/i)).toBeNull()
  expect(screen.getByRole('button', { name: /announce to teams/i })).toBeTruthy()
})

test('names the service on the announce button', () => {
  // Arrange
  withPullRequest({
    service: 'Teams',
    configured: true,
    channel: '#dev',
    channels: [],
    author: 'ana.lopez',
  })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByRole('button', { name: /announce to teams/i })).toBeTruthy()
})

test('offers to announce when a pull request exists', () => {
  // Arrange
  withPullRequest()

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByRole('button', { name: /announce to slack/i })).toBeTruthy()
})

test('says there is nothing to announce without a pull request', () => {
  // Arrange
  // makeSnapshot's review has no pull request found.
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      messaging: {
        service: 'Slack',
        configured: true,
        channel: '#dev',
        channels: [],
        author: 'ana.lopez',
      },
    }),
  })

  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText(/nothing to announce/i)).toBeTruthy()
  expect(screen.queryByRole('button', { name: /announce to slack/i })).toBeNull()
})

test('previews the message, then posts it on confirm', async () => {
  // Arrange
  const user = userEvent.setup()
  withPullRequest()
  render(<MessagingPanel />)

  // Act: open the preview, then confirm
  await user.click(screen.getByRole('button', { name: /announce to slack/i }))
  await screen.findByText('octocat announced the pull request')
  await user.click(screen.getByRole('button', { name: /post to slack/i }))

  // Assert
  expect(mockAnnounce).toHaveBeenCalledWith('#dev')
  expect(await screen.findByText(/announced/i)).toBeTruthy()
})

test('posts to the channel chosen in the preview', async () => {
  // Arrange
  const user = userEvent.setup()
  withPullRequest()
  render(<MessagingPanel />)

  // Act: open the preview, choose #releases, then confirm
  await user.click(screen.getByRole('button', { name: /announce to slack/i }))
  await screen.findByText('octocat announced the pull request')
  await user.selectOptions(screen.getByRole('combobox'), '#releases')
  await user.click(screen.getByRole('button', { name: /post to slack/i }))

  // Assert
  expect(mockAnnounce).toHaveBeenCalledWith('#releases')
})

test('posts to the first known channel when none is configured', async () => {
  // Arrange
  // A bot token with no configured channel: the preview comes back with an
  // empty channel, so the control must fall through to the first known one
  // rather than post to the empty channel the server would then reject.
  mockPreview.mockResolvedValueOnce({ text: 'octocat announced the pull request', channel: '' })
  const user = userEvent.setup()
  withPullRequest({
    service: 'Slack',
    configured: true,
    channel: '',
    channels: ['#dev', '#releases'],
    author: 'ana.lopez',
  })
  render(<MessagingPanel />)

  // Act: open the preview and confirm without touching the channel
  await user.click(screen.getByRole('button', { name: /announce to slack/i }))
  await screen.findByText('octocat announced the pull request')
  await user.click(screen.getByRole('button', { name: /post to slack/i }))

  // Assert
  expect(mockAnnounce).toHaveBeenCalledWith('#dev')
})

test('locks the confirm while a post is in flight', async () => {
  // Arrange
  // Hold the post open so the in-flight state is observable rather than
  // transient; a live confirm here would let a double click post twice.
  let releasePost = () => {}
  mockAnnounce.mockImplementationOnce(
    () =>
      new Promise<void>((resolve) => {
        releasePost = resolve
      }),
  )
  const user = userEvent.setup()
  withPullRequest()
  render(<MessagingPanel />)

  // Act: open the preview and confirm, leaving the post unresolved
  await user.click(screen.getByRole('button', { name: /announce to slack/i }))
  await screen.findByText('octocat announced the pull request')
  await user.click(screen.getByRole('button', { name: /post to slack/i }))

  // Assert: the confirm now reads "Posting…" and is disabled, and only one post fired
  const posting = await screen.findByRole('button', { name: /posting/i })
  expect(posting.hasAttribute('disabled')).toBe(true)
  expect(mockAnnounce).toHaveBeenCalledTimes(1)

  releasePost()
  await screen.findByText(/announced/i)
})

test('does not post if the preview is canceled', async () => {
  // Arrange
  const user = userEvent.setup()
  withPullRequest()
  render(<MessagingPanel />)

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
  render(<MessagingPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /announce to slack/i }))
  await screen.findByText('octocat announced the pull request')
  await user.click(screen.getByRole('button', { name: /post to slack/i }))

  // Assert
  expect(await screen.findByText(/could not be posted/i)).toBeTruthy()
})

test('prompts to connect before any snapshot arrives', () => {
  // Act
  render(<MessagingPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})
