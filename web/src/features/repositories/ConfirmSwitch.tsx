import { useEffect, useId, useRef } from 'react'
import type { Repositories } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { Failure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { useHoldShortcuts } from '@/features/keyboard/useShortcut.ts'

// Destination is a directory a switch was asked for, before it is confirmed.
export interface Destination {
  dir: string
  shown: string
}

interface ConfirmSwitchProps {
  destination: Destination
  switchTo: (dir: string) => Promise<Repositories>
  teller: Teller
  onCancel: () => void
}

// ConfirmSwitch asks once more before a switch, and makes it. It takes the
// focus as it opens, and is scrolled to: a switch asked from the picker, at
// the foot of the section, would otherwise ask out of sight.
export function ConfirmSwitch({ destination, switchTo, teller, onCancel }: ConfirmSwitchProps) {
  const headingId = useId()
  const heading = useRef<HTMLHeadingElement>(null)
  useHoldShortcuts()
  useEffect(() => {
    heading.current?.focus()
  }, [destination.dir])
  const go = useAsyncAction(() => switchTo(destination.dir), {
    fallback: 'The directory could not be switched to.',
    done: (switched) => `Switched to ${switched.here.shown}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <section
      aria-labelledby={headingId}
      className="flex flex-col gap-item rounded-lg border border-border p-4"
    >
      <h2
        id={headingId}
        ref={heading}
        tabIndex={-1}
        className="font-semibold focus-visible:outline-none"
      >
        Switch to <span className="font-mono">{destination.shown}</span>?
      </h2>
      <p className="text-sm text-muted-foreground">Every section is read again there.</p>
      {go.state === 'error' ? <Failure>{go.error}</Failure> : null}
      <div className="flex gap-item">
        <Button
          variant="primary"
          held={go.state === 'running'}
          onClick={() => {
            void go.run()
          }}
        >
          {go.state === 'running' ? 'Switching…' : 'Switch'}
        </Button>
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </section>
  )
}
