import type { Repositories } from '@/api/generated/types.gen.ts'
import { LastLook } from '@/lib/LastLook.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'

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

// ConfirmSwitch asks once more before a switch, and makes it. It is a section
// of its own whose heading takes the focus as it opens, and is scrolled to: a
// switch asked from the picker, at the foot of the section, would otherwise
// ask out of sight. Another directory picked while it asks asks again.
export function ConfirmSwitch({ destination, switchTo, teller, onCancel }: ConfirmSwitchProps) {
  const go = useAsyncAction(() => switchTo(destination.dir), {
    fallback: 'The directory could not be switched to.',
    done: (switched) => `Switched to ${switched.here.shown}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <LastLook
      key={destination.dir}
      section
      question={
        <>
          Switch to <span className="font-mono">{destination.shown}</span>?
        </>
      }
      cost="Every section is read again there."
      act="Switch"
      acting="Switching…"
      write={go}
      className="rounded-lg border border-border p-4"
      onAct={() => {
        void go.run()
      }}
      onCancel={onCancel}
    />
  )
}
