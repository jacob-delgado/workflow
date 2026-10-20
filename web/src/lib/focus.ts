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

// useFocusedControlInView brings a control that takes focus anywhere inside
// root wholly into view: each pane that scrolls it, innermost first and up to
// root itself, moves only as far as it must. Firefox scrolls a control that
// takes focus only when no more than a line of it shows, so a control a pane's
// edge cuts would stay cut, where Chromium and WebKit scroll it into view.
export function useFocusedControlInView(root: RefObject<HTMLElement | null>) {
  useEffect(() => {
    const outermost = root.current
    if (outermost === null) {
      return
    }

    return revealFocusIn(outermost)
  }, [root])
}

// revealFocusIn reveals each control that takes focus inside outermost until
// the returned stop is called, except two the browsers leave where they are: a
// control a press gives focus, which the pointer already shows and a scroll
// would move out from under it before the click lands, and the control taking
// focus back as the window does, which the user may have scrolled away from.
function revealFocusIn(outermost: HTMLElement): () => void {
  const cues = focusCues(outermost)
  const reveal = (event: FocusEvent) => {
    if (cues.revealing() && event.target instanceof Element) {
      revealInEveryPane(outermost, event.target)
    }
  }
  const stops = [
    listen(outermost, 'pointerdown', cues.press, { capture: true }),
    listen(window, 'click', cues.act, { capture: true }),
    listen(window, 'keydown', cues.act, { capture: true }),
    listen(window, 'blur', cues.leave),
    listen(outermost, 'focusin', reveal),
  ]

  return () => {
    for (const stop of stops) {
      stop()
    }
  }
}

// focusCues keeps what says how focus is moving inside outermost: a press
// holds until it clicks, and the window's leaving, with focus inside, until
// the one focus that comes back with it; a click or a key ends either, since
// any focus after one is the user's own.
function focusCues(outermost: HTMLElement) {
  let pressing = false
  let away = false

  return {
    press: () => {
      pressing = true
    },
    act: () => {
      pressing = false
      away = false
    },
    leave: (event: FocusEvent) => {
      away = event.target === window && outermost.contains(document.activeElement)
    },
    revealing: () => {
      const returning = away
      away = false

      return !pressing && !returning
    },
  }
}

function listen<K extends keyof WindowEventMap & keyof HTMLElementEventMap>(
  target: HTMLElement | Window,
  type: K,
  handler: (event: WindowEventMap[K] & HTMLElementEventMap[K]) => void,
  options?: AddEventListenerOptions,
): () => void {
  const listener = handler as EventListener
  target.addEventListener(type, listener, options)

  return () => {
    target.removeEventListener(type, listener, options)
  }
}

function revealInEveryPane(outermost: HTMLElement, control: Element) {
  let inner = control
  while (inner !== outermost && inner.parentElement !== null) {
    const pane = inner.parentElement
    if (scrolls(pane)) {
      revealInPane(pane, control)
    }
    inner = pane
  }
}

// scrolls says whether a pane scrolls its content up and down, which a pane
// whose overflow shows, or whose content fits, does not.
function scrolls(pane: HTMLElement): boolean {
  const { overflowY } = getComputedStyle(pane)

  return (overflowY === 'auto' || overflowY === 'scroll') && pane.scrollHeight > pane.clientHeight
}

// revealInPane scrolls the pane alone, and only as far as it must, to show the
// whole of a control in it clear of the pane's edges, or its head where the
// control is taller than the pane shows.
export function revealInPane(pane: HTMLElement, control: Element) {
  const box = control.getBoundingClientRect()
  const top = box.top - pane.getBoundingClientRect().top - pane.clientTop
  const bottom = top + box.height
  const { head, foot } = clearance(pane)
  const shown = pane.clientHeight - head - foot
  if (top < head || box.height > shown) {
    pane.scrollTop += top - head
  } else if (bottom > pane.clientHeight - foot) {
    pane.scrollTop += bottom - (pane.clientHeight - foot)
  }
}

// clearance is how far in from a pane's head and foot a control must sit to
// show: the larger of the pane's padding and its scroll padding at each, the
// scroll padding being how main keeps a field clear of the Settings Save bar.
function clearance(pane: HTMLElement): { head: number; foot: number } {
  const style = getComputedStyle(pane)

  return {
    head: Math.max(pixels(style.paddingTop), pixels(style.scrollPaddingTop)),
    foot: Math.max(pixels(style.paddingBottom), pixels(style.scrollPaddingBottom)),
  }
}

// pixels reads a computed length in pixels, and auto, as scroll padding can
// be, as none.
function pixels(length: string): number {
  return parseFloat(length) || 0
}
