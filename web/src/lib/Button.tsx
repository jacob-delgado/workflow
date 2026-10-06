import type { ComponentProps } from 'react'
import { cn } from './utils.ts'

// How each kind of button is drawn: primary for a section's one outward act —
// the commit, the push, the announcement — and secondary for every control
// beside one. Either shows it is off by color, whether the disabled attribute
// or aria-disabled turns it off. The primary's focus ring is the color of its
// fill, so a page-colored gap sets the ring apart from it.
const variants = {
  primary:
    'bg-primary font-medium focus-visible:ring-offset-2 focus-visible:ring-offset-background text-primary-foreground hover:bg-primary/90 disabled:bg-disabled disabled:text-disabled-foreground aria-disabled:bg-disabled aria-disabled:text-disabled-foreground',
  secondary:
    'border border-input text-foreground hover:bg-accent disabled:bg-disabled disabled:text-disabled-foreground aria-disabled:bg-disabled aria-disabled:text-disabled-foreground',
}

// How large a button is drawn: md for a section's acts, and sm for an act
// inline in a row of a list — a file's Stage, an issue's Switch branch, Load
// more under the list — so the row stays as tall as its words. The sizes
// match a field's, so a button and a field in one row line up.
const sizes = {
  md: 'px-3 py-1.5',
  sm: 'px-2 py-1',
}

const shared =
  'rounded-md text-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed aria-disabled:cursor-not-allowed'

interface ButtonProps extends ComponentProps<'button'> {
  variant: keyof typeof variants
  size?: keyof typeof sizes
}

// Button is a button drawn as a section's primary act or as a control beside
// one. It submits only when its type says so, so one in a form — a Cancel —
// never sends the form; className places it in its layout.
export function Button({
  variant,
  size = 'md',
  type = 'button',
  className,
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(shared, sizes[size], variants[variant], className)}
      {...props}
    />
  )
}
