import { Workflow } from 'lucide-react'
import { useEventStream } from '@/api/snapshot.ts'
import { NavRail } from './NavRail.tsx'
import { StreamStatus } from './StreamStatus.tsx'
import { sectionMeta } from './sections.ts'
import { useUiStore } from './uiStore.ts'

export function AppShell() {
  useEventStream()
  const section = useUiStore((state) => state.section)
  const { label } = sectionMeta[section]

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="flex items-center justify-between border-b border-border px-4 py-2.5">
        <span className="flex items-center gap-2 font-semibold tracking-tight">
          <Workflow aria-hidden className="size-5 text-primary" />
          workflow
        </span>
        <StreamStatus />
      </header>
      <div className="flex flex-1">
        <NavRail />
        <main className="flex-1 overflow-auto px-6 py-5">
          <h1 className="text-2xl font-semibold tracking-tight">{label}</h1>
          <p className="mt-2 text-muted-foreground">This section is coming together.</p>
        </main>
      </div>
    </div>
  )
}
