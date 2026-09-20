import { useEffect } from 'react'
import { create } from 'zustand'
import { zSnapshot } from './generated/zod.gen.ts'
import type { Snapshot } from './generated/types.gen.ts'

export type StreamStatus = 'connecting' | 'live' | 'stale'

interface SnapshotState {
  snapshot: Snapshot | null
  status: StreamStatus
}

// The latest full read state the event stream has pushed, and whether the
// stream is connected. The panels read from here; useEventStream is the only
// writer.
export const useSnapshotStore = create<SnapshotState>(() => ({
  snapshot: null,
  status: 'connecting',
}))

// useEventStream opens the Server-Sent Events connection once and keeps the
// snapshot store current from its pushes — the cockpit's freshness comes from
// the stream, never from polling. Mount it once, near the root. Each message is
// validated against the contract's schema; a payload that does not match is
// dropped rather than allowed to corrupt the last good snapshot.
export function useEventStream(): void {
  useEffect(() => {
    // `task web:mockup` sets VITE_MOCK so the whole cockpit can be navigated
    // against rich fixture data with no backend. The mock is code-split, so it
    // is never pulled into a production build.
    if (import.meta.env.VITE_MOCK === 'true') {
      void import('@/dev/mockSnapshot.ts').then((module) => {
        useSnapshotStore.setState({ snapshot: module.mockSnapshot, status: 'live' })
      })

      return
    }

    const source = new EventSource('/api/events')

    source.addEventListener('open', () => {
      useSnapshotStore.setState({ status: 'live' })
    })

    source.addEventListener('snapshot', (event) => {
      const message = event as MessageEvent<string>

      let raw: unknown
      try {
        raw = JSON.parse(message.data)
      } catch {
        return // a frame that is not even JSON is dropped, like a schema mismatch
      }

      const parsed = zSnapshot.safeParse(raw)
      if (parsed.success) {
        useSnapshotStore.setState({ snapshot: parsed.data, status: 'live' })
      }
    })

    source.addEventListener('error', () => {
      useSnapshotStore.setState({ status: 'stale' })
    })

    return () => {
      source.close()
    }
  }, [])
}
