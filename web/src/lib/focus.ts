import { useEffect, useRef, type RefObject } from 'react'

// useFocusOnMount gives focus to its element once, when it is first drawn: for
// a step that appears only because the user asked for it — a confirm, a
// preview, a form — so focus goes with them to it rather than falling to the
// page with the control that opened it.
export function useFocusOnMount<T extends HTMLElement>(): RefObject<T | null> {
  const target = useRef<T>(null)

  useEffect(() => {
    target.current?.focus()
  }, [])

  return target
}

// useFocusHandback hands focus back to the control that opened a step once the
// step is backed out of: call handBack as it closes, and the control, given the
// ref, takes focus when it is drawn again.
export function useFocusHandback<T extends HTMLElement>(): [RefObject<T | null>, () => void] {
  const target = useRef<T>(null)
  const pending = useRef(false)

  useEffect(() => {
    if (pending.current && target.current !== null) {
      pending.current = false
      target.current.focus()
    }
  })

  return [
    target,
    () => {
      pending.current = true
    },
  ]
}
