import { renderHook, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { FakeEventSource } from '@/test/fakeEventSource.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { useEventStream, useSnapshotStore } from './snapshot.ts'

const validSnapshot = makeSnapshot({ issues: { issues: [], total: 3, start_at: 0 } })

test('stores a snapshot the stream pushes and marks the stream live', () => {
  // Arrange
  renderHook(() => {
    useEventStream(null, vi.fn())
  })

  // Act
  FakeEventSource.latest().emit('snapshot', JSON.stringify(validSnapshot))

  // Assert
  expect(useSnapshotStore.getState().snapshot?.issues.total).toBe(3)
  expect(useSnapshotStore.getState().status).toBe('live')
})

test('keeps the scope a frame suggests for the next commit', () => {
  // Arrange
  renderHook(() => {
    useEventStream(null, vi.fn())
  })

  // Act
  FakeEventSource.latest().emit(
    'snapshot',
    JSON.stringify(makeSnapshot({ suggested_scope: 'api' })),
  )

  // Assert
  // A client generated before the field would strip it and the form would
  // never see the suggestion, so the stored snapshot must still carry it.
  expect(useSnapshotStore.getState().snapshot?.suggested_scope).toBe('api')
})

test.each([
  ['does not match the contract', JSON.stringify({ issues: 'not a page' })],
  ['is not JSON', 'not json{'],
])('a frame that %s leaves the last good snapshot, marked out of date and why', (_, frame) => {
  // Arrange
  renderHook(() => {
    useEventStream(null, vi.fn())
  })
  FakeEventSource.latest().emit('snapshot', JSON.stringify(validSnapshot))

  // Act
  FakeEventSource.latest().emit('snapshot', frame)

  // Assert
  const { snapshot, status, reason } = useSnapshotStore.getState()
  expect(snapshot?.issues.total).toBe(3)
  expect(status).toBe('stale')
  expect(reason).toMatch(/reload/i)
})

test('a good frame after a bad one brings the stream back to live', () => {
  // Arrange
  renderHook(() => {
    useEventStream(null, vi.fn())
  })
  FakeEventSource.latest().emit('snapshot', 'not json{')

  // Act
  FakeEventSource.latest().emit('snapshot', JSON.stringify(validSnapshot))

  // Assert
  expect(useSnapshotStore.getState()).toMatchObject({ status: 'live', reason: '' })
})

// A stale stream's reason is dropped by whatever happens to the stream next,
// so the header never pairs another state with "reload the page".
test.each([
  { event: 'error', status: 'reconnecting' },
  { event: 'open', status: 'live' },
])('a stream that goes $status after a bad frame says no reason', ({ event, status }) => {
  // Arrange
  renderHook(() => {
    useEventStream(null, vi.fn())
  })
  FakeEventSource.latest().emit('snapshot', 'not json{')

  // Act
  FakeEventSource.latest().emit(event, '')

  // Assert
  expect(useSnapshotStore.getState()).toMatchObject({ status, reason: '' })
})

test('a view chosen after a bad frame connects with no reason', () => {
  // Arrange
  const { rerender } = renderHook(
    ({ view }: { view: string | null }) => {
      useEventStream(view, vi.fn())
    },
    { initialProps: { view: null as string | null } },
  )
  FakeEventSource.latest().emit('snapshot', 'not json{')

  // Act
  rerender({ view: 'Team bugs' })

  // Assert
  expect(useSnapshotStore.getState()).toMatchObject({ status: 'connecting', reason: '' })
})

test('marks the stream live when it opens', () => {
  // Arrange
  renderHook(() => {
    useEventStream(null, vi.fn())
  })

  // Act
  FakeEventSource.latest().emit('open', '')

  // Assert
  expect(useSnapshotStore.getState().status).toBe('live')
})

test('marks the stream reconnecting when it errors', () => {
  // Arrange
  renderHook(() => {
    useEventStream(null, vi.fn())
  })

  // Act
  FakeEventSource.latest().emit('error', '')

  // Assert
  expect(useSnapshotStore.getState().status).toBe('reconnecting')
})

test('seeds mock data instead of connecting when VITE_MOCK is set', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderHook(() => {
    useEventStream(null, vi.fn())
  })

  // Assert
  await waitFor(() => {
    expect(useSnapshotStore.getState().snapshot?.review.found).toBe(true)
  })
  expect(FakeEventSource.instances).toHaveLength(0)
})

test('records the chosen view under VITE_MOCK', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderHook(() => {
    useEventStream('Team bugs', vi.fn())
  })

  // Assert
  await waitFor(() => {
    expect(useSnapshotStore.getState().view).toBe('Team bugs')
  })
})

test('opens the default view with no query', () => {
  // Act
  renderHook(() => {
    useEventStream(null, vi.fn())
  })

  // Assert
  expect(FakeEventSource.latest().url).toBe('/api/events')
})

test('reconnects with the chosen view', () => {
  // Arrange
  const { rerender } = renderHook(
    ({ view }: { view: string | null }) => {
      useEventStream(view, vi.fn())
    },
    { initialProps: { view: null as string | null } },
  )
  const first = FakeEventSource.latest()

  // Act
  rerender({ view: 'Team bugs' })

  // Assert
  expect(first.closed).toBe(true)
  expect(FakeEventSource.latest().url).toBe('/api/events?view=Team+bugs')
})

test('says it is connecting again while it reconnects', () => {
  // Arrange
  const { rerender } = renderHook(
    ({ view }: { view: string | null }) => {
      useEventStream(view, vi.fn())
    },
    { initialProps: { view: null as string | null } },
  )
  FakeEventSource.latest().emit('open', '')

  // Act
  rerender({ view: 'Team bugs' })

  // Assert
  expect(useSnapshotStore.getState().status).toBe('connecting')
})

test('records the view a snapshot was read for', () => {
  // Arrange
  renderHook(() => {
    useEventStream('Team bugs', vi.fn())
  })

  // Act
  FakeEventSource.latest().emit('snapshot', JSON.stringify(validSnapshot))

  // Assert
  expect(useSnapshotStore.getState().view).toBe('Team bugs')
})

test('hands a view the server refuses back to the caller', () => {
  // Arrange
  const onViewRefused = vi.fn()
  renderHook(() => {
    useEventStream('Removed view', onViewRefused)
  })

  // Act
  FakeEventSource.latest().refuse()

  // Assert
  expect(onViewRefused).toHaveBeenCalledOnce()
})

test('leaves a dropped view stream to reconnect on its own', () => {
  // Arrange
  const onViewRefused = vi.fn()
  renderHook(() => {
    useEventStream('Team bugs', onViewRefused)
  })

  // Act
  FakeEventSource.latest().emit('error', '')

  // Assert
  expect(onViewRefused).not.toHaveBeenCalled()
  expect(useSnapshotStore.getState().status).toBe('reconnecting')
})

test('marks a refused default stream reconnecting, with no view to hand back', () => {
  // Arrange
  const onViewRefused = vi.fn()
  renderHook(() => {
    useEventStream(null, onViewRefused)
  })

  // Act
  FakeEventSource.latest().refuse()

  // Assert
  expect(onViewRefused).not.toHaveBeenCalled()
  expect(useSnapshotStore.getState().status).toBe('reconnecting')
})
