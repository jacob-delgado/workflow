import { render, screen } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { SlackPanel } from './SlackPanel.tsx'

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

test('prompts to connect before any snapshot arrives', () => {
  // Act
  render(<SlackPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})
