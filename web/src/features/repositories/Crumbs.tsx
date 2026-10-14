import { cn } from '@/lib/utils.ts'
import type { Trail } from './dirFilter.ts'

// Crumbs are the directories above the one listed, each a button that lists
// it: the way up past where a typed path can go. The root's own slash
// stands between it and the next.
export function Crumbs({ trail, onGo }: { trail: Trail; onGo: (typed: string) => void }) {
  if (trail.above.length === 0) {
    return null
  }

  return (
    <nav aria-label="Up from here" className="flex flex-wrap items-center gap-1 px-4 pt-2 text-sm">
      {trail.above.map((above) => (
        <span key={above.typed} className="flex items-center gap-1">
          <button
            type="button"
            aria-label={`Up to ${above.typed === '/' ? '/' : above.typed.slice(0, -1)}`}
            onClick={() => {
              onGo(above.typed)
            }}
            className={cn(
              'rounded-sm font-mono text-primary underline-offset-2 hover:underline',
              'focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
            )}
          >
            {above.name}
          </button>
          {above.typed === '/' ? null : (
            <span aria-hidden className="text-muted-foreground">
              /
            </span>
          )}
        </span>
      ))}
      <span className="font-mono text-muted-foreground">{trail.name}</span>
    </nav>
  )
}
