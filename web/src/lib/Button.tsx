import type { ComponentProps } from 'react'
import { cn } from './utils.ts'

// How each kind of button is drawn, at one size: primary for a section's one
// outward act — the commit, the push, the announcement — and secondary for
// every control beside one. Either shows it is off by color, whether the
// disabled attribute or aria-disabled turns it off.
const variants = {
  primary:
    'rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground aria-disabled:cursor-not-allowed aria-disabled:bg-disabled aria-disabled:text-disabled-foreground',
  secondary:
    'rounded-md border border-input px-3 py-1.5 text-sm text-foreground hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground aria-disabled:cursor-not-allowed aria-disabled:bg-disabled aria-disabled:text-disabled-foreground',
}

interface ButtonProps extends ComponentProps<'button'> {
  variant: keyof typeof variants
}

// Button is a button drawn as a section's primary act or as a control beside
// one. It submits only when its type says so, so one in a form — a Cancel —
// never sends the form; className places it in its layout.
export function Button({ variant, type = 'button', className, ...props }: ButtonProps) {
  return <button type={type} className={cn(variants[variant], className)} {...props} />
}
