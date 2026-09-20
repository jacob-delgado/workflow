import type { ReactNode } from 'react'

// The resting state a panel shows when it has nothing yet — connecting, or a
// service the workspace has not configured. A quiet, bordered placeholder
// rather than a blank pane, so the section still reads as itself.
export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-40 items-center justify-center rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
      {children}
    </div>
  )
}
