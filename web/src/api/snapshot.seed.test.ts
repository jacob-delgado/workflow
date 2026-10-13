import { renderHook } from '@testing-library/react'
import { vi } from 'vitest'
import type { IssuesPage } from '@/api/generated/types.gen.ts'
import { FakeEventSource } from '@/test/setup/fakeEventSource.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { useEventStream, useSnapshotStore } from './snapshot.ts'

const kept: IssuesPage = {
  issues: [
    {
      key: 'PROJ-1',
      tracker: 'jira',
      summary: 'Kept from last time',
      status: 'To Do',
      status_category: 'new',
      type: 'Task',
    },
  ],
  total: 1,
  start_at: 0,
  unavailable: [],
}

// streaming opens the stream for view, as the page does.
function streaming(view: string | null) {
  return renderHook(
    ({ chosen }: { chosen: string | null }) => {
      useEventStream(chosen, vi.fn())
    },
    { initialProps: { chosen: view } },
  )
}

test('keeps the seed the stream sends ahead of its first snapshot, for the view it was opened for', () => {
  // Arrange
  streaming('Bugs')

  // Act
  FakeEventSource.latest().emit('seed', JSON.stringify({ issues: kept }))

  // Assert
  const { seed, snapshot, receivedAt } = useSnapshotStore.getState()
  expect(seed).toStrictEqual({ view: 'Bugs', page: kept })
  expect(snapshot).toBeNull()
  expect(receivedAt).toBe(0)
})

test("the first snapshot takes the seed's place", () => {
  // Arrange
  streaming(null)
  FakeEventSource.latest().emit('seed', JSON.stringify({ issues: kept }))

  // Act
  FakeEventSource.latest().emit('snapshot', JSON.stringify(makeSnapshot()))

  // Assert
  expect(useSnapshotStore.getState().seed).toBeNull()
})

test('a seed that does not match the contract is dropped, and the stream marked out of date', () => {
  // Arrange
  streaming(null)

  // Act
  FakeEventSource.latest().emit('seed', JSON.stringify({ issues: 'not a page' }))

  // Assert
  const { seed, status, reason } = useSnapshotStore.getState()
  expect(seed).toBeNull()
  expect(status).toBe('stale')
  expect(reason).toMatch(/reload/)
})

test('a snapshot the page cannot read drops the seed with it', () => {
  // Arrange
  streaming(null)
  FakeEventSource.latest().emit('seed', JSON.stringify({ issues: kept }))

  // Act
  FakeEventSource.latest().emit('snapshot', 'not json{')

  // Assert
  expect(useSnapshotStore.getState().seed).toBeNull()
})

test('a stream the browser gives up drops the seed', () => {
  // Arrange
  streaming(null)
  FakeEventSource.latest().emit('seed', JSON.stringify({ issues: kept }))

  // Act
  FakeEventSource.latest().refuse()

  // Assert
  const { seed, status } = useSnapshotStore.getState()
  expect(seed).toBeNull()
  expect(status).toBe('closed')
})

test('a stream that is reconnecting keeps the seed', () => {
  // Arrange
  streaming(null)
  FakeEventSource.latest().emit('seed', JSON.stringify({ issues: kept }))

  // Act
  FakeEventSource.latest().emit('error', '')

  // Assert
  expect(useSnapshotStore.getState().seed).toStrictEqual({ view: null, page: kept })
})

test("a stream opened for another view starts without the last view's seed", () => {
  // Arrange
  const { rerender } = streaming(null)
  FakeEventSource.latest().emit('seed', JSON.stringify({ issues: kept }))

  // Act
  rerender({ chosen: 'Bugs' })

  // Assert
  expect(useSnapshotStore.getState().seed).toBeNull()
})
