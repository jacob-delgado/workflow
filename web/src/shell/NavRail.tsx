import { useSnapshotStore } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'
import { sectionLabel, sectionMeta } from './sections.ts'
import { sections, useUiStore } from './uiStore.ts'

export function NavRail() {
  const section = useUiStore((state) => state.section)
  const setSection = useUiStore((state) => state.setSection)
  const service = useSnapshotStore((state) => state.snapshot?.messaging.service)

  return (
    <nav
      aria-label="Sections"
      className="flex w-20 shrink-0 flex-col gap-1 border-r border-border bg-card px-2 py-3"
    >
      {sections.map((key) => {
        const { Icon } = sectionMeta[key]
        const active = key === section

        return (
          <button
            key={key}
            type="button"
            aria-current={active ? 'page' : undefined}
            onClick={() => {
              setSection(key)
            }}
            className={cn(
              'flex flex-col items-center gap-1.5 rounded-md px-1 py-2 text-xs font-medium motion-safe:transition-colors',
              'text-muted-foreground hover:bg-accent hover:text-foreground',
              'focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
              active && 'bg-accent text-foreground',
            )}
          >
            <Icon aria-hidden className="size-5" />
            {sectionLabel(key, service)}
          </button>
        )
      })}
    </nav>
  )
}
