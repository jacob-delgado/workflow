import { ExternalLink } from 'lucide-react'
import type { ComponentProps } from 'react'
import { cn } from './utils.ts'

type NewTabLinkProps = Omit<ComponentProps<'a'>, 'target' | 'rel'> & { href: string }

// isWebAddress is whether href leads to a page on the web, over http or https:
// the only addresses a link drawn from what a server sends may lead to. Any
// other scheme — data:, a script, a file share — and anything that is no
// address at all is not one.
export function isWebAddress(href: string): boolean {
  try {
    const { protocol } = new URL(href)

    return protocol === 'http:' || protocol === 'https:'
  } catch {
    return false
  }
}

// NewTabLink is a link out of workflow — to the forge, the tracker — that opens
// in a new tab, and says so before it is followed: drawn in the link color with
// the external-link icon, and named with "(opens in a new tab)" for a screen
// reader, so it never reads as static text, and nobody is moved to a new tab
// unwarned. className places it in its layout. Its address comes from a
// server, so one that is not a web address — none at all, among them — draws
// the link's words alone, as plain text that nothing styles as a link.
export function NewTabLink({ children, className, href, ...props }: NewTabLinkProps) {
  if (!isWebAddress(href)) {
    return <span>{children}</span>
  }

  return (
    <a
      {...props}
      href={href}
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
