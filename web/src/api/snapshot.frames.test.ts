import { renderHook } from '@testing-library/react'
import { vi } from 'vitest'
import frames from '@/test/snapshot-frames.sse?raw'
import { FakeEventSource } from '@/test/setup/fakeEventSource.ts'
import { zSeed, zSnapshot } from './generated/zod.gen.ts'
import { useEventStream, useSnapshotStore } from './snapshot.ts'

// The frames the server's stream writes, held byte for byte by
// internal/webserver's TestStreamFrameMatchesTheClientGolden, which rewrites
// them with -update. A snapshot the server changes without the client being
// regenerated fails here, rather than in a browser that drops the frame.

// A message the stream writes: its event and its data, as EventSource hands
// them to a listener.
interface Message {
  event: string
  data: string
}

// messages is each message the stream wrote, in order: the stream's messages
// are separated by a blank line.
function messages(stream: string): Message[] {
  return stream
    .split('\n\n')
    .map((block) => {
      const lines = block.split('\n')
      const field = (name: string) =>
        lines.find((line) => line.startsWith(`${name}: `))?.slice(`${name}: `.length)

      return { event: field('event'), data: field('data') }
    })
    .filter((read): read is Message => read.event !== undefined && read.data !== undefined)
}

const written = messages(frames)
const seeds = written.filter((message) => message.event === 'seed').map((message) => message.data)
const workspaces = ['a filled workspace', 'an empty workspace']
const served = written
  .filter((message) => message.event === 'snapshot')
  .map((message, index) => [workspaces[index] ?? `frame ${String(index)}`, message.data])

test('the server wrote a seed, then a frame for each workspace', () => {
  // Act
  const events = written.map((message) => message.event)

  // Assert
  expect(events).toStrictEqual(['seed', 'snapshot', 'snapshot'])
})

test.each(seeds)("the client's schema keeps every key of the seed", (data) => {
  // Arrange
  const raw: unknown = JSON.parse(data)

  // Act
  const parsed = zSeed.parse(raw)

  // Assert
  expect(parsed).toStrictEqual(raw)
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

test.each(seeds)(
  'the stream keeps the seed the server writes, ahead of its first snapshot',
  (data) => {
    // Arrange
    renderHook(() => {
      useEventStream(null, vi.fn())
    })

    // Act
    FakeEventSource.latest().emit('seed', data)

    // Assert
    const parsed: unknown = JSON.parse(data)
    expect(useSnapshotStore.getState().seed?.page).toStrictEqual(
      (parsed as { issues: unknown }).issues,
    )
  },
)
