import { useEffect, useState } from 'react'
import { LastLook } from '@/lib/LastLook.tsx'
import { useUiStore } from '@/shell/uiStore.ts'

// useGuardLeaving tells the shell whether the form holds unsaved edits, so a
// change of section asks before it drops them, until the form goes.
function useGuardLeaving(unsaved: boolean): void {
  const setUnsaved = useUiStore((state) => state.setUnsaved)

  useEffect(() => {
    setUnsaved(unsaved)

    return () => {
      setUnsaved(false)
    }
  }, [unsaved, setUnsaved])
}

// LeaveUnsaved guards a change of section while the form holds unsaved edits:
// once another section is asked for over them, it asks whether to leave
// without saving them, and draws nothing until one is.
export function LeaveUnsaved({ unsaved }: { unsaved: number }) {
  useGuardLeaving(unsaved > 0)
  const leaving = useUiStore((state) => state.leaving)

  return leaving === null ? null : <LeaveQuestion unsaved={unsaved} />
}

// LeaveQuestion is the question itself. Stay hands the focus back to where it
// was when the section was asked for, as the question opened over it.
function LeaveQuestion({ unsaved }: { unsaved: number }) {
  const leave = useUiStore((state) => state.leave)
  const stay = useUiStore((state) => state.stay)
  const [asker] = useState(() => document.activeElement)
  const changes = unsaved === 1 ? '1 unsaved change' : `${String(unsaved)} unsaved changes`

  return (
    <LastLook
      question="Leave without saving?"
      cost={`Leaving drops ${changes}.`}
      act="Leave"
      cancel="Stay"
      onAct={leave}
      onCancel={() => {
        stay()
        if (asker instanceof HTMLElement) {
          asker.focus()
        }
      }}
    />
  )
}
