import { Lock, Workflow } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useHealth, useHealthStore } from '@/api/health.ts'
import { useEventStream, useSnapshotStore } from '@/api/snapshot.ts'
import { useRefreshViews } from '@/features/issues/issueApi.ts'
import { cn } from '@/lib/utils.ts'
import { NavRail } from './NavRail.tsx'
import { SectionPanel } from './SectionPanel.tsx'
import { StreamStatus } from './StreamStatus.tsx'
import { ThemeToggle } from './ThemeToggle.tsx'
import { sectionLabel, sectionMeta } from './sections.ts'
import { useApplyTheme } from './useApplyTheme.ts'
import { useUiStore, type Section } from './uiStore.ts'

export function AppShell() {
  const view = useUiStore((state) => state.view)
  const setView = useUiStore((state) => state.setView)
  const refreshViews = useRefreshViews()
  useEventStream(view, () => {
    // The server has no such view (a restart dropped it from the
    // configuration): go back to its default, and read again what it offers.
    setView(null)
    refreshViews()
  })
  useHealth()
  useApplyTheme()
  const health = useHealthStore((state) => state.health)
  const section = useUiStore((state) => state.section)
  const main = useSectionFocus(section)
  const service = useSnapshotStore((state) => state.snapshot?.messaging.service)

  // The shell is the window's height and never scrolls: the header and the
  // rail hold still, and the content scrolls beneath the header — or, where a
  // section splits into panes, each pane scrolls on its own. Every link in the
  // chain down to the scrolling part sets min-h-0, or it would grow to its
  // content and scroll the page instead. And a scrolling part that holds text
  // kept for a screen reader (sr-only, drawn out of the flow) is positioned,
  // so that text is placed, scrolled and clipped inside it: placed against the
  // window, it would grow the page.
  return (
    <div className="flex h-dvh flex-col">
      <a
        href="#main"
        className="sr-only focus-visible:not-sr-only focus-visible:absolute focus-visible:left-4 focus-visible:top-2 focus-visible:z-10 focus-visible:rounded-md focus-visible:bg-card focus-visible:px-3 focus-visible:py-1.5 focus-visible:text-sm focus-visible:ring-2 focus-visible:ring-ring"
      >
        Skip to content
      </a>
      <header className="flex flex-wrap items-center justify-between gap-x-group gap-y-tight border-b border-border px-4 py-2.5">
        <span className="flex items-center gap-2 font-semibold tracking-tight">
          <Workflow aria-hidden className="size-5 text-primary" />
          workflow
          {health ? (
            <span className="text-xs font-normal text-muted-foreground tabular-nums">
              {health.version}
            </span>
          ) : null}
        </span>
        <div className="flex items-center gap-item">
          <ThemeToggle />
          <StreamStatus />
        </div>
      </header>
      {health?.dry_run ? (
        <p
          role="status"
          className="flex items-center gap-2 border-b border-border bg-muted px-4 py-2 text-sm"
        >
          <Lock aria-hidden className="size-4 shrink-0 text-muted-foreground" />
          <span>
            Read-only: started with <code className="font-mono">--dry-run</code>; every write is
            held back.
          </span>
        </p>
      ) : null}
      <div className="flex min-h-0 flex-1">
        <NavRail />
        <main
          ref={main}
          id="main"
          tabIndex={-1}
          className="relative flex flex-1 flex-col gap-block overflow-auto p-block focus-visible:outline-none"
        >
          <SectionHeading section={section} service={service} />
          <SectionPanel section={section} />
        </main>
      </div>
    </div>
  )
}

// SectionHeading names the section in its system's hue, beside the icon the
// rail shows for it, so the heading and the rail's active item read as one.
// The icon takes the hue itself rather than inheriting it: under reduced
// motion every property transitions for 0.01ms (web/src/index.css), and an
// inherited color reaches an icon's strokes a frame after the heading's text.
function SectionHeading({ section, service }: { section: Section; service: string | undefined }) {
  const { Icon, hue } = sectionMeta[section]

  return (
    <h1 className={cn('flex items-center gap-2 text-2xl', hue)}>
      <Icon aria-hidden className={cn('size-6 shrink-0', hue)} />
      {sectionLabel(section, service)}
    </h1>
  )
}

// useSectionFocus hands focus to the content area whenever the section changes —
// from the nav rail or a work-story stage — so a keyboard or screen reader user
// lands in the section they chose rather than being left in the rail, or on a
// button the change took away. The section opens at its top, rather than as far
// down as the last one was scrolled. The first section is no change: on load,
// focus stays where the browser puts it.
function useSectionFocus(section: Section) {
  const main = useRef<HTMLElement>(null)
  const shown = useRef(section)

  useEffect(() => {
    const content = main.current
    if (shown.current === section || content === null) {
      return
    }

    shown.current = section
    content.scrollTop = 0
    content.focus()
  }, [section])

  return main
}
