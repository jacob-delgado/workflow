import { useRef, type RefObject } from 'react'
import { cn } from '@/lib/utils.ts'
import { facetLabel, isPicked, type Facet, type FacetChoice } from './reviewFacets.ts'

interface FacetChipsProps {
  choices: FacetChoice[]
  picked: Facet[]
  onToggle: (facet: Facet) => void
  // afterFilter takes focus when the last button goes, and the group with it.
  afterFilter: RefObject<HTMLElement | null>
}

// FacetChips narrows the review queue, as the terminal's `f` does: a button
// per repository, CI state, draft or ready, and author the queue holds, each
// with how many requests hold it, pressed while it narrows the queue.
export function FacetChips({ choices, picked, onToggle, afterFilter }: FacetChipsProps) {
  const group = useRef<HTMLDivElement>(null)

  if (choices.length === 0) {
    return null
  }

  return (
    <div
      ref={group}
      tabIndex={-1}
      role="group"
      aria-label="Filter"
      className="flex flex-wrap items-center gap-tight text-sm"
    >
      <span aria-hidden="true" className="text-muted-foreground">
        Filter
      </span>
      {choices.map((choice) => {
        const pressed = isPicked(picked, choice.facet)

        return (
          <button
            key={`${choice.facet.kind}:${choice.facet.value}`}
            type="button"
            aria-pressed={pressed}
            onClick={() => {
              // Unpicking a value no request holds removes its button, so
              // focus stays in the group rather than falling to the page —
              // or, when it was the group's last, goes on past it.
              if (pressed && choice.count === 0) {
                const stays = choices.length === 1 ? afterFilter : group
                stays.current?.focus()
              }
              onToggle(choice.facet)
            }}
            className={cn(
              'rounded-md border px-2 py-0.5 text-foreground hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
              pressed ? 'border-foreground bg-accent font-medium' : 'border-input',
            )}
          >
            {`${facetLabel(choice.facet)} ${String(choice.count)}`}
          </button>
        )
      })}
    </div>
  )
}
