import type { ReactNode } from 'react'
import { Tabs as Base } from '@base-ui/react/tabs'
import { cn } from '@/lib/utils.ts'

interface TabsProps {
  value: string
  onValueChange: (value: string) => void
  className?: string
  children: ReactNode
}

// Tabs shows one panel at a time, chosen from a list of tabs beside them; the
// list runs down its side, so Up and Down move between its tabs, and Enter or
// a press shows one.
export function Tabs({ value, onValueChange, className, children }: TabsProps) {
  return (
    <Base.Root
      value={value}
      onValueChange={(chosen: unknown) => {
        onValueChange(String(chosen))
      }}
      orientation="vertical"
      className={className}
    >
      {children}
    </Base.Root>
  )
}

// TabList is the tabs, named by label.
export function TabList({
  label,
  className,
  children,
}: {
  label: string
  className?: string
  children: ReactNode
}) {
  return (
    <Base.List aria-label={label} className={className}>
      {children}
    </Base.List>
  )
}

// Tab is one tab, which shows the panel of its value; the one shown is marked
// by its ground and an edge in the ink.
export function Tab({ value, children }: { value: string; children: ReactNode }) {
  return (
    <Base.Tab
      value={value}
      className={cn(
        'relative flex items-center justify-between gap-item rounded-md px-3 py-1.5 text-left text-sm text-muted-foreground',
        'hover:bg-accent/60 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
        'aria-selected:bg-accent aria-selected:text-foreground',
      )}
    >
      {children}
    </Base.Tab>
  )
}

// TabPanel is what one tab shows. It stays in the page while another is shown,
// hidden, so a form spread over the panels keeps every field of it. It is no
// stop of Tab's itself: each panel holds controls, the first of which Tab
// reaches next.
export function TabPanel({
  value,
  className,
  children,
}: {
  value: string
  className?: string
  children: ReactNode
}) {
  return (
    <Base.Panel
      value={value}
      keepMounted
      tabIndex={-1}
      className={cn('focus-visible:outline-none', className)}
    >
      {children}
    </Base.Panel>
  )
}
