import { renderHook } from '@testing-library/react'
import { FakeEventSource } from '@/test/fakeEventSource.ts'
import { useEventStream, useSnapshotStore } from './snapshot.ts'

const validSnapshot = {
  issues: { issues: [], total: 3, start_at: 0 },
  branch: {
    name: 'fix/PROJ-1',
    detached: false,
    head: 'abc123',
    upstream: '',
    ahead: 0,
    behind: 0,
    base: 'origin/main',
    commits: [],
  },
  changes: { changes: [] },
  review: { found: false },
  slack: { channel: '#dev', channels: [], author: 'octocat' },
}

test('stores a snapshot the stream pushes and marks the stream live', () => {
  // Arrange
  renderHook(() => {
    useEventStream()
  })

  // Act
  FakeEventSource.latest().emit('snapshot', JSON.stringify(validSnapshot))

  // Assert
  expect(useSnapshotStore.getState().snapshot?.issues.total).toBe(3)
  expect(useSnapshotStore.getState().status).toBe('live')
})

test('drops a payload that does not match the contract', () => {
  // Arrange
  renderHook(() => {
    useEventStream()
  })

  // Act
  FakeEventSource.latest().emit('snapshot', JSON.stringify({ issues: 'not a page' }))

  // Assert
  expect(useSnapshotStore.getState().snapshot).toBeNull()
})

test('marks the stream live when it opens', () => {
  // Arrange
  renderHook(() => {
    useEventStream()
  })

  // Act
  FakeEventSource.latest().emit('open', '')

  // Assert
  expect(useSnapshotStore.getState().status).toBe('live')
})

test('marks the stream stale when it errors', () => {
  // Arrange
  renderHook(() => {
    useEventStream()
  })

  // Act
  FakeEventSource.latest().emit('error', '')

  // Assert
  expect(useSnapshotStore.getState().status).toBe('stale')
})
