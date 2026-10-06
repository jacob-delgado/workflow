import { Children, type ReactNode } from 'react'
import { cn } from './utils.ts'

// Meta is a row's facts, each an element of its own — so a branch can be code,
// a date a <time>, and each fact styled or read alone — spaced apart, with the
// middle dot drawn between them. The dot is hidden from assistive tech, which
// hears the facts as separate words rather than "middle dot". A fact left out
// (null, false or undefined) takes no place and no dot.
export function Meta({ children, className }: { children: ReactNode; className?: string }) {
  const facts = Children.toArray(children)

  return (
    <span className={cn('inline-flex flex-wrap items-baseline gap-x-item', className)}>
      {facts.map((fact, place) => (
        <span key={place} className="inline-flex items-baseline gap-x-item">
          {place === 0 ? null : (
            <>
              <span aria-hidden="true">·</span>{' '}
            </>
          )}
          <span>{fact}</span>
        </span>
      ))}
    </span>
  )
}
