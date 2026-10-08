import { Children, isValidElement, type ReactNode } from 'react'
import { cn, keyedByText } from './utils.ts'

// Meta is a row's facts, each an element of its own — so a branch can be code,
// a date a <time>, and each fact styled or read alone — spaced apart, with the
// middle dot drawn between them. The dot is hidden from assistive tech, which
// hears the facts as separate words rather than "middle dot". A fact left out
// (null, false or undefined) takes no place and no dot.
export function Meta({ children, className }: { children: ReactNode; className?: string }) {
  const facts = keyedByText(Children.toArray(children), factText)

  return (
    <span className={cn('inline-flex flex-wrap items-baseline gap-x-item', className)}>
      {facts.map(({ key, item: fact }, place) => (
        <span key={key} className="inline-flex items-baseline gap-x-item">
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

// factText is what a fact is known by among its row's: an element by the key
// Children.toArray gave it, a word by its text.
function factText(fact: ReturnType<typeof Children.toArray>[number]): string {
  if (isValidElement(fact)) {
    return String(fact.key)
  }

  return typeof fact === 'string' || typeof fact === 'number' ? String(fact) : ''
}

// CodeValue is a definition list's value, written as code, or "None" in the
// muted foreground when it is missing: one word for absence in every list.
export function CodeValue({ value }: { value: string }) {
  return value === '' ? (
    <dd className="text-muted-foreground">None</dd>
  ) : (
    <dd className="font-mono">{value}</dd>
  )
}
