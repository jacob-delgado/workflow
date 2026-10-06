import { useEffect, useRef, type RefObject } from 'react'
import { create } from 'zustand'
import { useShallow } from 'zustand/shallow'
import { sections, type Section } from '@/shell/uiStore.ts'
import { jumpAction, paneSections } from './boundActions.ts'
import { ariaShortcuts } from './keyNames.ts'
import { keysOf, useKeysStore } from './keysApi.ts'

// How a control answers its action: a button is pressed, and a field — a
// search, a select, the comment box — takes the focus, as the terminal's key
// opens it for typing.
export type Answer = 'click' | 'focus'

// A control on screen that answers one of the terminal's actions.
export interface Registration {
  action: string
  target: RefObject<HTMLElement | null>
  answer: Answer
}

// useRegistry is every control drawn now that answers an action, in the order
// they were drawn, and how many confirms drawn now hold the single keys. Only
// the section shown draws its controls, so an action is answered where its
// terminal pane would answer it.
export const useRegistry = create<{ registered: Registration[]; holds: number }>(() => ({
  registered: [],
  holds: 0,
}))

// useHoldShortcuts holds every single-key shortcut while the confirm or
// preview calling it is drawn: it asks a question of its own, and a key
// pressed while it is open would otherwise press a control behind it.
export function useHoldShortcuts(): void {
  useEffect(() => {
    useRegistry.setState(({ holds }) => ({ holds: holds + 1 }))

    return () => {
      useRegistry.setState(({ holds }) => ({ holds: holds - 1 }))
    }
  }, [])
}

// shortcutsHeld reports a confirm or a preview open, holding the single keys.
export function shortcutsHeld(): boolean {
  return useRegistry.getState().holds > 0
}

// useShortcut registers the control target holds as the one that answers
// action while it is drawn, and returns its aria-keyshortcuts: the action's
// keys while single-key shortcuts are on, and none while they are off or the
// server lists none. An undefined action registers nothing, for a control
// drawn where it answers none.
export function useShortcut(
  action: string | undefined,
  target: RefObject<HTMLElement | null>,
  answer: Answer = 'click',
): string | undefined {
  useEffect(() => {
    if (action === undefined) {
      return undefined
    }

    const registration = { action, target, answer }
    useRegistry.setState(({ registered }) => ({ registered: [...registered, registration] }))

    return () => {
      useRegistry.setState(({ registered }) => ({
        registered: registered.filter((each) => each !== registration),
      }))
    }
  }, [action, target, answer])

  return useKeysStore((state) =>
    state.shortcuts && action !== undefined
      ? ariaShortcuts(keysOf(state.actions, action))
      : undefined,
  )
}

// useShortcutProps is useShortcut for a control with no ref of its own: the
// ref and the aria-keyshortcuts to spread on it.
export function useShortcutProps<T extends HTMLElement>(
  action: string | undefined,
  answer: Answer = 'click',
): { ref: RefObject<T | null>; 'aria-keyshortcuts': string | undefined } {
  const ref = useRef<T>(null)

  return { ref, 'aria-keyshortcuts': useShortcut(action, ref, answer) }
}

// usePaneShortcuts is each section's aria-keyshortcuts in the rail: the
// terminal's pane numbers that open it, while single-key shortcuts are on.
export function usePaneShortcuts(): Partial<Record<Section, string>> {
  return useKeysStore(
    useShallow((state) => {
      const numbers = state.shortcuts ? keysOf(state.actions, jumpAction) : []

      return Object.fromEntries(
        sections.map((section) => [
          section,
          ariaShortcuts(numbers.filter((_, index) => paneSections[index] === section)),
        ]),
      )
    }),
  )
}

// usable reports a registered control that is drawn and on: one that is off,
// by disabled or by aria-disabled, would only refuse the press.
function usable(registration: Registration): boolean {
  const element = registration.target.current

  return (
    element !== null && element.isConnected && !element.matches(':disabled, [aria-disabled="true"]')
  )
}

// liveRegistrations is each action's first usable control drawn now.
export function liveRegistrations(): Registration[] {
  const seen = new Set<string>()

  return useRegistry.getState().registered.filter((registration) => {
    if (seen.has(registration.action) || !usable(registration)) {
      return false
    }

    seen.add(registration.action)

    return true
  })
}

// answerWith does what a control answers its action with: the press a click
// makes, so a confirm or a preview it opens still asks; or the focus.
export function answerWith(registration: Registration): void {
  const element = registration.target.current
  if (registration.answer === 'focus') {
    element?.focus()
  } else {
    element?.click()
  }
}
