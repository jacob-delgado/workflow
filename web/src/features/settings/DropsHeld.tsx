import { type ReactNode, useState } from 'react'
import type { QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { LastLook } from '@/lib/LastLook.tsx'

// useHeldForCi is the announcement the server holds until a pull request's CI
// passes, while it waits, or undefined. A save reopens the server with the
// file, as a switch does, which drops it unposted, so Settings asks first.
function useHeldForCi(): QueuedAnnouncement | undefined {
  const held = useSnapshotStore((state) => state.snapshot?.queued_announcement)

  return held?.state === 'waiting' ? held : undefined
}

// useDropsHeldWords says what a save costs the announcement waiting for CI,
// or nothing while none waits.
export function useDropsHeldWords(): string {
  const held = useHeldForCi()
  const { sigil } = useForgeWords()
  if (held === undefined) {
    return ''
  }

  return `Saving reopens workflow --web with the file, as a switch does, which drops the announcement waiting for ${sigil}${String(held.pull)}'s CI, unposted. `
}

// AskedSave is a save that asks first while an announcement waits for CI:
// submit saves the values at once, or draws question, which saves them once
// asked; question is null while nothing asks.
interface AskedSave<T> {
  submit: (values: T) => Promise<void>
  question: ReactNode
}

// useAskedSave is save, asking first while an announcement waits for CI.
export function useAskedSave<T>(save: (values: T) => Promise<void>): AskedSave<T> {
  const held = useHeldForCi()
  const [asking, setAsking] = useState<{ values: T } | null>(null)
  const submit = (values: T) => {
    if (held === undefined) {
      return save(values)
    }
    setAsking({ values })

    return Promise.resolve()
  }
  const question =
    asking === null ? null : (
      <DropsHeld
        onSave={() => {
          setAsking(null)
          void save(asking.values)
        }}
        onCancel={() => {
          setAsking(null)
        }}
      />
    )

  return { submit, question }
}

interface DropsHeldProps {
  onSave: () => void
  onCancel: () => void
}

// DropsHeld asks before a save that drops the announcement waiting for CI.
function DropsHeld({ onSave, onCancel }: DropsHeldProps) {
  const cost = useDropsHeldWords()

  return (
    <LastLook
      question="Save, and drop the announcement waiting for CI?"
      cost={cost}
      act="Save"
      onAct={onSave}
      onCancel={onCancel}
    />
  )
}
