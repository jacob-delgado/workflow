import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { create } from 'zustand'
import { getKeys } from '@/api/generated'
import { getKeysQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { KeyAction, KeyList } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build.

interface KeysState {
  // Whether a single key pressed outside a text field acts (ui.web_shortcuts).
  shortcuts: boolean
  // Every action the terminal's help lists, ui.keys applied, in its order.
  actions: KeyAction[]
}

// useKeysStore holds the keys the server last served, for every control that
// names its key in aria-keyshortcuts and for the keyboard layer that answers
// a press: a store rather than a query, so a control drawn on its own — in a
// test, or before the read lands — names no key rather than reading the
// server itself. Until the read lands there are no keys, and the single keys
// are off.
export const useKeysStore = create<KeysState>(() => ({ shortcuts: false, actions: [] }))

// readKeys reads the actions and the shortcut setting. Under VITE_MOCK it
// serves the fixture (code-split, dev-only), so the mockup's keys work with no
// backend.
async function readKeys(signal: AbortSignal): Promise<KeyList> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockKeys } = await import('@/dev/mockKeys.ts')

    return mockKeys
  }

  const { data } = await getKeys({ signal, throwOnError: true })

  return data
}

// useReadKeys reads the keys once, and again whenever a save or a read of the
// configuration invalidates them, into useKeysStore. A read that fails leaves
// the page with no keys of its own: Tab, the mouse and the palette's sections
// still reach everything.
export function useReadKeys(): void {
  const { data } = useQuery({
    queryKey: getKeysQueryKey(),
    queryFn: ({ signal }) => readKeys(signal),
    retry: false,
  })

  useEffect(() => {
    if (data !== undefined) {
      useKeysStore.setState({ shortcuts: data.single_key_shortcuts, actions: data.actions })
    }
  }, [data])
}

// keysOf is the keys an action is bound on, none where it is not listed.
export function keysOf(actions: KeyAction[], action: string): string[] {
  return actions.find((listed) => listed.action === action)?.keys ?? []
}
