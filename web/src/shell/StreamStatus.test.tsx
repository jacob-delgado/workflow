import { act, render, screen } from '@testing-library/react'
import { onTestFinished, vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { StreamStatus } from './StreamStatus.tsx'

test('reflects the live stream state', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'live' })

  // Act
  render(<StreamStatus />)

  // Assert
  expect(screen.getByRole('status').textContent).toContain('Live')
})

test('tells a dropped stream apart from one whose frames this page cannot read', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'stale', reason: 'a frame did not match; reload the page' })

  // Act
  render(<StreamStatus />)

  // Assert
  const region = screen.getByRole('status')
  expect(region.textContent).toContain('Out of date')
  expect(region.textContent).toContain('a frame did not match; reload the page')
})

test('says a dropped stream is reconnecting', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'reconnecting' })

  // Act
  render(<StreamStatus />)

  // Assert
  expect(screen.getByRole('status').textContent).toContain('Reconnecting')
})

test('says a stream the browser closed for good is disconnected, and to reload', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'closed', reason: 'it stopped; reload the page' })

  // Act
  render(<StreamStatus />)

  // Assert
  const region = screen.getByRole('status')
  expect(region.textContent).toContain('Disconnected')
  expect(region.textContent).toContain('reload the page')
})

test.each([
  ['connecting', 'not-started'],
  ['live', 'done'],
  ['reconnecting', 'in-flight'],
  ['stale', 'failed'],
  ['closed', 'failed'],
] as const)('draws a %s stream as the %s mark, beside its words', (status, mark) => {
  // Arrange
  useSnapshotStore.setState({ status })

  // Act
  render(<StreamStatus />)

  // Assert
  expect(markShape(screen.getByRole('status'))).toBe(drawnMark(mark))
})

test('says when the last snapshot landed beside the pill, and keeps it current', () => {
  // Arrange
  vi.useFakeTimers()
  onTestFinished(() => {
    vi.useRealTimers()
  })
  useSnapshotStore.setState({ status: 'live', receivedAt: Date.now() - 3_000 })
  render(<StreamStatus />)

  // Act
  act(() => {
    vi.advanceTimersByTime(2_000)
  })

  // Assert
  // Outside the live region, so the count is not spoken every second.
  expect(screen.getByText('Updated 5s ago')).toBeTruthy()
  expect(screen.getByRole('status').textContent).not.toContain('Updated')
})

test('says nothing of an update before the first snapshot lands', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'connecting', receivedAt: 0 })

  // Act
  render(<StreamStatus />)

  // Assert
  expect(screen.queryByText(/^Updated/)).toBeNull()
})
