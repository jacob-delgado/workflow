import { type ReactNode, useSyncExternalStore } from 'react'
import { Tabs as Base } from '@base-ui/react/tabs'
import { cn } from '@/lib/utils.ts'

// The width from which the tab list stands beside its panels, as Tailwind's lg
// does; below it, the tabs wrap in rows above them.
const besideQuery = '(min-width: 1024px)'

// useBeside reports whether the window is wide enough for the tabs to stand
// beside their panels, following it as it changes.
function useBeside(): boolean {
  return useSyncExternalStore(
    (changed) => {
      const media = window.matchMedia(besideQuery)
      media.addEventListener('change', changed)

      return () => {
        media.removeEventListener('change', changed)
      }
    },
    () => window.matchMedia(besideQuery).matches,
  )
}

interface TabsProps {
  value: string
  onValueChange: (value: string) => void
  children: ReactNode
}

// Tabs is a set of panels shown one at a time, chosen from a list of tabs: a
// column beside the panels in a wide window, read with the up and down
// arrows, and rows above them in a narrower one, read with left and right.
export function Tabs({ value, onValueChange, children }: TabsProps) {
  const beside = useBeside()

  return (
    <Base.Root
      value={value}
      onValueChange={(chosen: unknown) => {
        onValueChange(String(chosen))
      }}
      orientation={beside ? 'vertical' : 'horizontal'}
      className="flex flex-col gap-section lg:flex-row lg:items-start"
    >
      {children}
    </Base.Root>
  )
}

// TabList is the tabs, named for what they choose between.
export function TabList({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Base.List
      aria-label={label}
      className="flex flex-wrap gap-1 lg:w-52 lg:shrink-0 lg:flex-col lg:flex-nowrap"
    >
      {children}
    </Base.List>
  )
}

interface TabProps {
  value: string
  // className sets a tab apart from those before it.
  className?: string
  children: ReactNode
}

// Tab is one tab, drawn as the rail draws a section: muted until chosen.
export function Tab({ value, className, children }: TabProps) {
  return (
    <Base.Tab
      value={value}
      className={cn(
        'flex items-center justify-between gap-item rounded-md px-3 py-1.5 text-left text-sm font-medium motion-safe:transition-colors',
        'text-muted-foreground hover:bg-accent hover:text-foreground',
        'focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
        'data-active:bg-accent data-active:text-foreground',
        className,
      )}
    >
      {children}
    </Base.Tab>
  )
}

// TabPanel is the panel a tab shows. Every panel stays in the page while
// another is shown, hidden, so what is typed in one outlives a move to
// another. A panel is no Tab stop of its own: each holds controls, the first
// of which Tab reaches next.
export function TabPanel({ value, children }: { value: string; children: ReactNode }) {
  return (
    <Base.Panel
      value={value}
      keepMounted
      tabIndex={-1}
      className="flex min-w-0 flex-col gap-section"
    >
      {children}
    </Base.Panel>
  )
}
