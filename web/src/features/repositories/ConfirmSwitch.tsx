import type { Repositories } from '@/api/generated/types.gen.ts'
import { LastLook } from '@/lib/LastLook.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'

// Destination is a directory a switch was asked for, before it is confirmed.
export interface Destination {
  dir: string
  shown: string
}

// Drawn is where a switch is asked: in a section of its own, under a heading,
// where the opener stood; or as a step of the directory dialog, in place of
// its list, which Back returns to.
type Drawn = 'section' | 'step'

interface ConfirmSwitchProps {
  destination: Destination
  switchTo: (dir: string) => Promise<Repositories>
  teller: Teller
  drawn: Drawn
  // onCancel goes back to what asked: the opener, or the dialog's list.
  onCancel: () => void
  // onSwitched hears that the switch was made, after the teller says so.
  onSwitched?: () => void
}

// ConfirmSwitch asks once more before a switch, and makes it. The question
// takes the focus as it opens, so it is heard wherever it is drawn. Another
// directory chosen asks again.
export function ConfirmSwitch({
  destination,
  switchTo,
  teller,
  drawn,
  onCancel,
  onSwitched,
}: ConfirmSwitchProps) {
  const go = useAsyncAction(() => switchTo(destination.dir), {
    fallback: 'The directory could not be switched to.',
    done: (switched) => `Switched to ${switched.here.shown}.`,
    onStart: teller.clear,
    onDone: (said) => {
      teller.say(said)
      onSwitched?.()
    },
  })

  return (
    <LastLook
      key={destination.dir}
      section={drawn === 'section'}
      question={
        <>
          Switch to <span className="font-mono">{destination.shown}</span>?
        </>
      }
      cost="Every section is read again there."
      act="Switch"
      acting="Switching…"
      cancel={drawn === 'step' ? 'Back' : 'Cancel'}
      write={go}
      className={drawn === 'step' ? 'p-block' : 'rounded-lg border border-border p-4'}
      onAct={() => {
        void go.run()
      }}
      onCancel={onCancel}
    />
  )
}
