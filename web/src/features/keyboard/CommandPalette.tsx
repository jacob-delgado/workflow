import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import type { LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils.ts'
import { ModalDialog } from './ModalDialog.tsx'
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
function rank(entries: PaletteEntry[], typed: string): PaletteEntry[] {
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

  return groups.flatMap((group) => closest.filter((entry) => entry.group === group))
}

// CommandPalette finds an action by name: a combobox over the section's
// actions, by the terminal's words for them, and the sections to go to. Up
// and down choose, Enter runs the chosen one — through its own control, so a
// confirm it opens still asks — and Escape closes.
export function CommandPalette({ section, entries, onClose, onRun }: CommandPaletteProps) {
  const ids = useId()
  const input = useRef<HTMLInputElement>(null)
  const [typed, setTyped] = useState('')
  const [chosen, setChosen] = useState(0)
  const shown = rank(entries, typed)
  const active = Math.min(chosen, shown.length - 1)
  const optionId = (index: number) => `${ids}-option-${String(index)}`

  useEffect(() => {
    document.getElementById(optionId(active))?.scrollIntoView({ block: 'nearest' })
  })

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    const step = { ArrowDown: 1, ArrowUp: -1 }[event.key]
    if (step !== undefined && shown.length > 0) {
      event.preventDefault()
      setChosen((active + step + shown.length) % shown.length)
    } else if (event.key === 'Enter' && shown[active] !== undefined) {
      event.preventDefault()
      onRun(shown[active])
    }
  }

  return (
    <ModalDialog
      label="Command palette"
      takesFocus={input}
      onClose={onClose}
      className="mx-auto mt-[12dvh] mb-auto w-[34rem] overflow-hidden shadow-lg"
    >
      {/* A rule in the section's hue across the top, as its heading wears it,
          says which section's actions these are. */}
      <div aria-hidden className={cn('h-0.5 bg-current', section.hue)} />
      <div className="flex items-center gap-3 border-b border-border px-4">
        <section.Icon aria-hidden className={cn('size-5 shrink-0', section.hue)} />
        <input
          ref={input}
          role="combobox"
          aria-label="Action"
          aria-expanded
          aria-controls={`${ids}-list`}
          aria-autocomplete="list"
          aria-activedescendant={shown.length > 0 ? optionId(active) : undefined}
          value={typed}
          placeholder={`Find an action in ${section.name}`}
          autoComplete="off"
          spellCheck={false}
          onChange={(event) => {
            setTyped(event.target.value)
            setChosen(0)
          }}
          onKeyDown={onKeyDown}
          className="min-w-0 flex-1 bg-transparent py-3.5 text-lg placeholder:text-muted-foreground focus-visible:outline-none"
        />
      </div>
      <Options
        listId={`${ids}-list`}
        shown={shown}
        active={active}
        hue={section.hue}
        optionId={optionId}
        onRun={onRun}
      />
      {shown.length === 0 ? (
        <p role="status" className="px-4 py-3 text-sm text-muted-foreground">
          Nothing here is called that. Try fewer words.
        </p>
      ) : null}
    </ModalDialog>
  )
}

interface OptionsProps {
  listId: string
  shown: PaletteEntry[]
  active: number
  hue: string
  optionId: (index: number) => string
  onRun: (entry: PaletteEntry) => void
}

// Options is the listbox of what matches, under each group's name. The chosen
// one is marked in the section's hue; a press on one runs it, and the focus
// stays in the box.
function Options({ listId, shown, active, hue, optionId, onRun }: OptionsProps) {
  const groups = [...new Set(shown.map((entry) => entry.group))]

  return (
    <div
      id={listId}
      role="listbox"
      aria-label="Actions"
      className="max-h-[min(24rem,60dvh)] overflow-y-auto py-1.5"
    >
      {groups.map((group) => (
        <div key={group} role="group" aria-label={group} className="py-1">
          <div aria-hidden className="px-4 pt-1 pb-0.5 text-xs text-muted-foreground">
            {group}
          </div>
          {shown.map((entry, index) =>
            entry.group === group ? (
              // An option never has the focus — the combobox keeps it, and its own
              // keys choose and run one — so a press is its only listener.
              // eslint-disable-next-line jsx-a11y/click-events-have-key-events
              <div
                key={entry.id}
                id={optionId(index)}
                role="option"
                aria-selected={index === active}
                tabIndex={-1}
                onMouseDown={(event) => {
                  event.preventDefault()
                }}
                onClick={() => {
                  onRun(entry)
                }}
                className={cn(
                  'relative flex cursor-pointer items-center justify-between gap-group px-4 py-1.5 text-sm',
                  index === active ? 'bg-accent' : 'hover:bg-accent/60',
                )}
              >
                {index === active ? (
                  <span
                    aria-hidden
                    className={cn('absolute inset-y-1 left-0 w-0.5 bg-current', hue)}
                  />
                ) : null}
                <span>{entry.words}</span>
                {entry.key === undefined ? null : <Key>{entry.key}</Key>}
              </div>
            ) : null,
          )}
        </div>
      ))}
    </div>
  )
}
