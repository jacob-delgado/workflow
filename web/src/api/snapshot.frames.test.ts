import { renderHook } from '@testing-library/react'
import { vi } from 'vitest'
import frames from '@/test/snapshot-frames.sse?raw'
import { FakeEventSource } from '@/test/fakeEventSource.ts'
import { zSnapshot } from './generated/zod.gen.ts'
import { useEventStream, useSnapshotStore } from './snapshot.ts'

// The frames the server's stream writes, held byte for byte by
// internal/webserver's TestStreamFrameMatchesTheClientGolden, which rewrites
// them with -update. A snapshot the server changes without the client being
// regenerated fails here, rather than in a browser that drops the frame.

// payloads is each snapshot event's data, as EventSource hands it to a
// listener: the stream's events are separated by a blank line.
function payloads(stream: string): string[] {
  return stream
    .split('\n\n')
    .map((event) => event.split('\n').find((line) => line.startsWith('data: ')))
    .filter((line) => line !== undefined)
    .map((line) => line.slice('data: '.length))
}

const workspaces = ['a filled workspace', 'an empty workspace']
const served = payloads(frames).map((data, index) => [
  workspaces[index] ?? `frame ${String(index)}`,
  data,
])

test('the server wrote a frame for each workspace', () => {
  // Assert
  expect(served.map(([name]) => name)).toEqual(workspaces)
})

test.each(served)('the stream shows the frame the server writes for %s, and is live', (_, data) => {
  // Arrange
  renderHook(() => {
    useEventStream(null, vi.fn())
  })

  // Act
  FakeEventSource.latest().emit('snapshot', data)

  // Assert
  const { snapshot, status } = useSnapshotStore.getState()
  expect(status).toBe('live')
  expect(snapshot).toStrictEqual(JSON.parse(data))
})

test.each(served)("the client's schema keeps every key of the frame for %s", (_, data) => {
  // Arrange
  // zod strips a key it does not know rather than refusing the frame, so a
  // field the server added and the client never learned would vanish in
  // silence; only a round trip that loses nothing proves the two agree.
  const raw: unknown = JSON.parse(data)

  // Act
  const parsed = zSnapshot.parse(raw)

  // Assert
  expect(parsed).toStrictEqual(raw)
})
