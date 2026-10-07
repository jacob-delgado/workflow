import { ExternalLink } from 'lucide-react'
import type { ComponentProps } from 'react'
import { cn } from './utils.ts'

type NewTabLinkProps = Omit<ComponentProps<'a'>, 'target' | 'rel'> & { href: string }

// NewTabLink is a link out of workflow — to the forge, the tracker — that opens
// in a new tab, and says so before it is followed: drawn in the link color with
// the external-link icon, and named with "(opens in a new tab)" for a screen
// reader, so it never reads as static text, and nobody is moved to a new tab
// unwarned. className places it in its layout.
export function NewTabLink({ children, className, ...props }: NewTabLinkProps) {
  return (
    <a
      {...props}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        'inline-flex items-center gap-1.5 text-primary underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
        className,
      )}
    >
      {children}
      <ExternalLink aria-hidden className="size-3.5 shrink-0" />{' '}
      <span className="sr-only">(opens in a new tab)</span>
    </a>
  )
}
