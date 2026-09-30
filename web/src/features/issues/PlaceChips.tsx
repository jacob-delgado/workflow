import { useRef } from 'react'
import { cn } from '@/lib/utils.ts'
import { samePlace, type Place, type PlaceChoice } from './issuePlaces.ts'

interface PlaceChipsProps {
  choices: PlaceChoice[]
  picked: Place[]
  onToggle: (place: Place) => void
}

// PlaceChips narrows the issue list to places, as the terminal's `p` does: a
// button per status the loaded issues are in and per mark, each with how many
// issues it holds, pressed while it narrows the list. The name and count are
// one text, so a chip never reads as the row mark it shares a word with.
export function PlaceChips({ choices, picked, onToggle }: PlaceChipsProps) {
  const group = useRef<HTMLDivElement>(null)

  if (choices.length === 0) {
    return null
  }

  return (
    <div
      ref={group}
      tabIndex={-1}
      role="group"
      aria-label="Where"
      className="flex flex-wrap items-center gap-tight text-sm"
    >
      <span aria-hidden="true" className="text-muted-foreground">
        Where
      </span>
      {choices.map((choice) => {
        const pressed = picked.some((place) => samePlace(place, choice.place))

        return (
          <button
            key={`${choice.place.kind}:${choice.place.name}`}
            type="button"
            aria-pressed={pressed}
            onClick={() => {
              // Unpicking a place no issue is in removes its button, so focus
              // stays in the group rather than falling to the page.
              if (pressed && choice.count === 0) {
                group.current?.focus()
              }
              onToggle(choice.place)
            }}
            className={cn(
              'rounded-md border px-2 py-0.5 text-foreground hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
              pressed ? 'border-foreground bg-accent font-medium' : 'border-input',
            )}
          >
            {`${choice.place.name} ${String(choice.count)}`}
          </button>
        )
      })}
    </div>
  )
}
