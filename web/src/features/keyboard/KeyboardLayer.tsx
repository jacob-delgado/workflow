import { useEffect, useState } from 'react'
import { flushSync } from 'react-dom'
import type { KeyAction } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { shortcutsHeld } from '@/lib/LastLook.tsx'
import { sectionLabel, sectionMeta } from '@/shell/sections.ts'
import { sections, useUiStore, type Section } from '@/shell/uiStore.ts'
import { helpAction, jumpAction, paneSections } from './boundActions.ts'
import { CommandPalette, type PaletteEntry } from './CommandPalette.tsx'
import { opensPalette, plainKey, pressable, takesTyping } from './keyNames.ts'
import { keysOf, useKeysStore, useReadKeys } from './keysApi.ts'
import { ShortcutSheet } from './ShortcutSheet.tsx'
import { answerWith, liveRegistrations } from './useShortcut.ts'

type Open = 'sheet' | 'palette' | null

// helpKeyOf is the key the sheet opens on: the terminal's toggle-help key where
// the page can bind it, and ? where it cannot.
function helpKeyOf(actions: KeyAction[]): string {
  return keysOf(actions, helpAction).find(pressable) ?? '?'
}

// jumpFor is the section a pane number opens, or undefined for a key that is
// none.
function jumpFor(actions: KeyAction[], key: string): Section | undefined {
  const index = keysOf(actions, jumpAction).indexOf(key)

  return index < 0 ? undefined : paneSections[index]
}

// answerKey does what a single key does where it was pressed: goes to the
// section its pane number names, or answers through the control on screen
// bound to it. It reports whether the key did anything.
function answerKey(key: string): boolean {
  const { actions } = useKeysStore.getState()
  const section = jumpFor(actions, key)
  if (section !== undefined) {
    useUiStore.getState().setSection(section)

    return true
  }

  const bound = liveRegistrations().find((registration) =>
    keysOf(actions, registration.action).includes(key),
  )
  if (bound !== undefined) {
    answerWith(bound)
  }

  return bound !== undefined
}

// paletteEntries is what the palette offers in a section: each action a
// control on screen answers, in the terminal's words and order, then each
// other section to go to.
function paletteEntries(section: Section, service: string | undefined): PaletteEntry[] {
  const { shortcuts, actions } = useKeysStore.getState()
  const live = liveRegistrations()
  const keyOf = (keys: string[]) => (shortcuts ? keys.find(pressable) : undefined)
  const here = sectionLabel(section, service)
  const answered = actions.flatMap((action) => {
    const registration = live.find((each) => each.action === action.action)

    return registration === undefined
      ? []
      : [
          {
            id: action.action,
            words: action.help,
            group: here,
            key: keyOf(action.keys),
            run: () => {
              answerWith(registration)
            },
          },
        ]
  })
  const elsewhere = sections
    .filter((other) => other !== section)
    .map((other) => ({
      id: `go-${other}`,
      words: sectionLabel(other, service),
      group: 'Go to',
      key: keyOf(keysOf(actions, jumpAction).filter((_, index) => paneSections[index] === other)),
      run: () => {
        useUiStore.getState().setSection(other)
      },
    }))

  return [...answered, ...elsewhere]
}

// KeyboardLayer is the page's keyboard beyond Tab: ? opens the sheet of keys,
// Control+K or Command+K the palette, and — while single-key shortcuts are on
// — a key the terminal binds does here what it does there. No single key acts
// while a field, a select or the comment composer has the focus, while the
// sheet or the palette is open, or while a confirm or a preview is.
export function KeyboardLayer() {
  useReadKeys()
  const [open, setOpen] = useState<Open>(null)
  const section = useUiStore((state) => state.section)
  const setSection = useUiStore((state) => state.setSection)
  const service = useSnapshotStore((state) => state.snapshot?.messaging.service)
  const helpKey = useKeysStore((state) => helpKeyOf(state.actions))

  useEffect(() => {
    if (open !== null) {
      return undefined
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) {
        return
      }

      if (opensPalette(event)) {
        event.preventDefault()
        setOpen('palette')

        return
      }

      const key = plainKey(event)
      if (key === undefined || takesTyping(event.target)) {
        return
      }

      if (key === helpKey) {
        event.preventDefault()
        setOpen('sheet')
      } else if (useKeysStore.getState().shortcuts && !shortcutsHeld() && answerKey(key)) {
        event.preventDefault()
      }
    }

    window.addEventListener('keydown', onKeyDown)

    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [open, helpKey])

  const close = () => {
    setOpen(null)
  }

  if (open === 'sheet') {
    return (
      <ShortcutSheet
        helpKey={helpKey}
        onClose={close}
        onSettings={() => {
          close()
          setSection('settings')
        }}
      />
    )
  }

  if (open === 'palette') {
    return (
      <CommandPalette
        section={{ name: sectionLabel(section, service), ...sectionMeta[section] }}
        entries={paletteEntries(section, service)}
        onClose={close}
        onRun={(entry) => {
          // The palette closes, handing the focus back, before the choice runs,
          // so a step it opens takes the focus from there.
          flushSync(close)
          entry.run()
        }}
      />
    )
  }

  return null
}
