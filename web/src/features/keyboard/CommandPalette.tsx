import { useRef, useState } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Autocomplete, type AutocompleteGroup } from '@/lib/baseui/Autocomplete.tsx'
import { Dialog } from '@/lib/baseui/Dialog.tsx'
import { cn } from '@/lib/utils.ts'
import { Key } from './ShortcutSheet.tsx'

// One thing the palette can do: its words, the group it is listed under, the
// key that does it too, where one does, and what doing it runs.
export interface PaletteEntry {
  id: string
  words: string
  group: string
  key?: string
  run: () => void
}

// The section the palette opens over: its name, rail icon and hue.
interface PaletteSection {
  name: string
  Icon: LucideIcon
  hue: string
}

interface CommandPaletteProps {
  section: PaletteSection
  entries: PaletteEntry[]
  onClose: () => void
  onRun: (entry: PaletteEntry) => void
}

// rank is the entries that hold every word typed, in any order and any case,
// closest first: one whose words are what was typed, then those that start
// with it, then the rest, each as listed. They stay under their groups, the
// group of the closest first, so the order Up and Down step through is the
// order they are drawn in.
function rank(entries: PaletteEntry[], typed: string): AutocompleteGroup<PaletteEntry>[] {
  const wanted = typed.trim().toLowerCase()
  const terms = wanted.split(/\s+/).filter(Boolean)
  const words = (entry: PaletteEntry) => entry.words.toLowerCase()
  const closeness = (entry: PaletteEntry) => {
    if (words(entry) === wanted) {
      return 0
    }

    return words(entry).startsWith(wanted) ? 1 : 2
  }
  const closest = entries
    .filter((entry) => terms.every((term) => words(entry).includes(term)))
    .toSorted((one, other) => closeness(one) - closeness(other))
  const groups = [...new Set(closest.map((entry) => entry.group))]

  return groups.map((group) => ({
    name: group,
    items: closest.filter((entry) => entry.group === group),
  }))
}

// CommandPalette finds an action by name: a combobox over the section's
// actions, by the terminal's words for them, and the sections to go to. Up
// and down choose, Enter runs the chosen one — through its own control, so a
// confirm it opens still asks — and Escape closes.
export function CommandPalette({ section, entries, onClose, onRun }: CommandPaletteProps) {
  const input = useRef<HTMLInputElement>(null)
  const [typed, setTyped] = useState('')

  return (
    <Dialog
      label="Command palette"
      initialFocus={input}
      onClose={onClose}
      className="mt-[12dvh] w-[34rem] self-start overflow-hidden shadow-lg"
    >
      {/* A rule in the section's hue across the top, as its heading wears it,
          says which section's actions these are. */}
      <div aria-hidden className={cn('h-0.5 bg-current', section.hue)} />
      <Autocomplete
        groups={rank(entries, typed)}
        typed={typed}
        onType={setTyped}
        label="Action"
        listLabel="Actions"
        placeholder={`Find an action in ${section.name}`}
        inputRef={input}
        itemKey={(entry) => entry.id}
        itemText={(entry) => entry.words}
        renderItem={(entry) => <Option entry={entry} hue={section.hue} />}
        onChoose={onRun}
        empty="Nothing here is called that. Try fewer words."
        leading={<section.Icon aria-hidden className={cn('size-5 shrink-0', section.hue)} />}
      />
    </Dialog>
  )
}

// Option is one action as the palette lists it: its words, and its key where
// one does it too. The highlighted one is marked in the section's hue.
function Option({ entry, hue }: { entry: PaletteEntry; hue: string }) {
  return (
    <>
      <span
        aria-hidden
        className={cn(
          'absolute inset-y-1 left-0 hidden w-0.5 bg-current group-data-highlighted:block',
          hue,
        )}
      />
      <span>{entry.words}</span>
      {entry.key === undefined ? null : <Key>{entry.key}</Key>}
    </>
  )
}
