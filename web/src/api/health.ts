import { useEffect, useRef } from 'react'
import { create } from 'zustand'
import { getHealth } from './generated/sdk.gen.ts'
import type { Health } from './generated/types.gen.ts'
import { useSnapshotStore } from './snapshot.ts'

interface HealthState {
  health: Health | null
}

// The server's build and mode — its version, and whether it was started with
// --dry-run — read when the cockpit mounts and again whenever the event stream
// comes back after dropping. It is null until the first read lands, and a failed
// read leaves it as it was: with none, the cockpit shows no version and no
// banner, and the server's own guard still refuses every write under dry run.
export const useHealthStore = create<HealthState>(() => ({ health: null }))

// useHealth reads the server's health on mount, and again when the stream
// reconnects after a drop: the server listens on a fixed port, so the one a tab
// reconnects to may be a restart in the other mode. Mount it once, near the
// root, beside the event stream.
export function useHealth(): void {
  const status = useSnapshotStore((state) => state.status)
  const dropped = useRef(false)

  useEffect(() => {
    void loadHealth()
  }, [])

  useEffect(() => {
    if (status === 'stale') {
      dropped.current = true
    } else if (status === 'live' && dropped.current) {
      dropped.current = false
      void loadHealth()
    }
  }, [status])
}

async function loadHealth(): Promise<void> {
  // Under VITE_MOCK there is no server to ask; the mockup is a writable build.
  if (import.meta.env.VITE_MOCK === 'true') {
    useHealthStore.setState({ health: { version: 'mockup', dry_run: false } })

    return
  }

  try {
    const result = await getHealth({ throwOnError: true })
    useHealthStore.setState({ health: result.data })
  } catch {
    // Unreadable health leaves the store as it was; see useHealthStore.
  }
}
