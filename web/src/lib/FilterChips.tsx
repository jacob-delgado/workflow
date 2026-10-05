import { useRef, type RefObject } from 'react'
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
  onToggle: (value: V) => void
  // afterLast takes focus when the group's last button goes, and the group with
  // it; without one, focus stays on the group.
  afterLast?: RefObject<HTMLElement | null>
}

// FilterChips narrows a list, as the terminal's checklists do: a button per
// value the list holds, each with how many hold it, pressed while it narrows
// the list. The name and count are one text, so a chip never reads as a row's
// mark it shares a word with.
export function FilterChips<V>({
  label,
  choices,
  isPicked,
  nameOf,
  keyOf,
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
              'rounded-md border px-2 py-0.5 text-foreground hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
              pressed ? 'border-foreground bg-accent font-medium' : 'border-input',
            )}
          >
            {`${nameOf(choice.value)} ${String(choice.count)}`}
          </button>
        )
      })}
    </div>
  )
}
