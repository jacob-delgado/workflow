import { Workflow } from 'lucide-react'
import { useEventStream } from '@/api/snapshot.ts'
import { NavRail } from './NavRail.tsx'
import { SectionPanel } from './SectionPanel.tsx'
import { StreamStatus } from './StreamStatus.tsx'
import { ThemeToggle } from './ThemeToggle.tsx'
import { sectionMeta } from './sections.ts'
import { useApplyTheme } from './useApplyTheme.ts'
import { useUiStore } from './uiStore.ts'

export function AppShell() {
  useEventStream()
  useApplyTheme()
  const section = useUiStore((state) => state.section)
  const { label } = sectionMeta[section]

  return (
    <div className="flex min-h-dvh flex-col">
      <a
        href="#main"
        className="sr-only focus-visible:not-sr-only focus-visible:absolute focus-visible:left-4 focus-visible:top-2 focus-visible:z-10 focus-visible:rounded-md focus-visible:bg-card focus-visible:px-3 focus-visible:py-1.5 focus-visible:text-sm focus-visible:ring-2 focus-visible:ring-ring"
      >
        Skip to content
      </a>
      <header className="flex items-center justify-between border-b border-border px-4 py-2.5">
        <span className="flex items-center gap-2 font-semibold tracking-tight">
          <Workflow aria-hidden className="size-5 text-primary" />
          workflow
        </span>
        <div className="flex items-center gap-3">
          <ThemeToggle />
          <StreamStatus />
        </div>
      </header>
      <div className="flex flex-1">
        <NavRail />
        <main
          id="main"
          tabIndex={-1}
          className="flex-1 overflow-auto px-6 py-5 focus-visible:outline-none"
        >
          <h1 className="text-2xl font-semibold tracking-tight">{label}</h1>
          <SectionPanel section={section} />
        </main>
      </div>
    </div>
  )
}
