import { render, screen } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

const pull = {
  number: 128,
  url: 'https://forge.example.com/pull/128',
  title: 'Redact tokens in the request log',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean' as const,
}

test('shows the pull request and its CI checks', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        pull,
        ci: {
          state: 'running',
          total: 2,
          done: 1,
          failed: 1,
          checks: [
            { name: 'build', state: 'passed', url: 'https://forge.example.com/build' },
            { name: 'e2e', state: 'failed', url: '' },
          ],
        },
      },
    }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByRole('link', { name: /redact tokens/i })).toBeTruthy()
  expect(screen.getByText('build')).toBeTruthy()
  expect(screen.getByText('e2e')).toBeTruthy()
})

test('shows a pull request that has no CI', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: true, pull } }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: /redact tokens/i })).toBeTruthy()
})

test('says when there is no open pull request', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByText(/no open pull request/i)).toBeTruthy()
})

test('prompts to connect before any snapshot arrives', () => {
  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})
