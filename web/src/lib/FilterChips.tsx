import { useRef, type ReactElement, type RefObject } from 'react'
import { cn } from '@/lib/utils.ts'

// FilterChoice is a value a list can be narrowed to, with how many of the
// listed things hold it.
export interface FilterChoice<V> {
  value: V
  count: number
}

interface FilterChipsProps<V> {
  // label names the group and leads it, as the terminal's checklist titles it.
  label: string
  choices: FilterChoice<V>[]
  isPicked: (value: V) => boolean
  nameOf: (value: V) => string
  keyOf: (value: V) => string
  // markOf is the mark a value's chip draws before its name — the one the
  // list's rows draw for it — or undefined for none. It is hidden from
  // assistive tech, so the chip is named by its name and count alone.
  markOf?: (value: V) => ReactElement | undefined
  onToggle: (value: V) => void
  // afterLast takes focus when the group's last button goes, and the group with
  // it; without one, focus stays on the group.
  afterLast?: RefObject<HTMLElement | null>
}

// FilterChips narrows a list, as the terminal's checklists do: a button per
// value the list holds, each with how many hold it, after the mark its rows
// draw for it where they draw one, pressed — in the tonal wash, edged in its
// text color, which holds 3:1 against the page, and twice as thick as an
// unpressed chip's edge, so pressed reads by weight as well as by color —
// while it narrows the list. The name and count are one text, so a chip never
// reads as a row's mark it shares a word with.
export function FilterChips<V>({
  label,
  choices,
  isPicked,
  nameOf,
  keyOf,
  markOf,
  onToggle,
  afterLast,
}: FilterChipsProps<V>) {
  const group = useRef<HTMLDivElement>(null)

  if (choices.length === 0) {
    return null
  }

  return (
    <div
      ref={group}
      tabIndex={-1}
      role="group"
      aria-label={label}
      className="flex flex-wrap items-center gap-tight text-sm"
    >
      <span aria-hidden="true" className="text-muted-foreground">
        {label}
      </span>
      {choices.map((choice) => {
        const pressed = isPicked(choice.value)
        const mark = markOf?.(choice.value)

        return (
          <button
            key={keyOf(choice.value)}
            type="button"
            aria-pressed={pressed}
            onClick={() => {
              // Unpicking a value nothing holds removes its button, so focus
              // stays in the group rather than falling to the page — or, when
              // it was the group's last, goes on past it.
              if (pressed && choice.count === 0) {
                const stays = choices.length === 1 && afterLast !== undefined ? afterLast : group
                stays.current?.focus()
              }
              onToggle(choice.value)
            }}
            className={cn(
              'inline-flex items-center gap-1 rounded-md border px-2 py-0.5 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none motion-safe:transition-colors',
              pressed
                ? // The thicker edge takes its width from the padding, so a
                  // chip keeps its size as it is pressed.
                  'border-2 border-tonal-foreground bg-tonal px-1.75 py-px font-medium text-tonal-foreground hover:bg-tonal-hover'
                : 'border-input text-foreground hover:bg-accent',
            )}
          >
            {mark === undefined ? null : (
              <span aria-hidden="true" className="flex shrink-0">
                {mark}
              </span>
            )}
            {`${nameOf(choice.value)} ${String(choice.count)}`}
          </button>
        )
      })}
    </div>
  )
}
