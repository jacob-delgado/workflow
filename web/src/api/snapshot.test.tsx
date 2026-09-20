import { renderHook, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { FakeEventSource } from '@/test/fakeEventSource.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { useEventStream, useSnapshotStore } from './snapshot.ts'

const validSnapshot = makeSnapshot({ issues: { issues: [], total: 3, start_at: 0 } })

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

test('drops a snapshot frame that is not valid JSON', () => {
  // Arrange
  renderHook(() => {
    useEventStream()
  })

  // Act
  FakeEventSource.latest().emit('snapshot', 'not json{')

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

test('seeds mock data instead of connecting when VITE_MOCK is set', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderHook(() => {
    useEventStream()
  })

  // Assert
  await waitFor(() => {
    expect(useSnapshotStore.getState().snapshot?.review.found).toBe(true)
  })
  expect(FakeEventSource.instances).toHaveLength(0)
})
