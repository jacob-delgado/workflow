import { useSnapshotStore } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'
import { sectionLabel, sectionMeta } from './sections.ts'
import { sections, useUiStore } from './uiStore.ts'

// NavRail is the sections, each an icon over its name. Below md the rail keeps
// only the icons, to leave the content the width: each name stays in its
// button as text a screen reader reads, so the buttons are named alike at
// every width, and as the title a pointer resting on the icon shows.
export function NavRail() {
  const section = useUiStore((state) => state.section)
  const setSection = useUiStore((state) => state.setSection)
  const service = useSnapshotStore((state) => state.snapshot?.messaging.service)

  return (
    <nav
      aria-label="Sections"
      className="flex w-14 shrink-0 flex-col gap-tight border-r border-border bg-card px-2 py-3 md:w-20"
    >
      {sections.map((key) => {
        const { Icon, hue } = sectionMeta[key]
        const active = key === section
        const name = sectionLabel(key, service)

        return (
          <button
            key={key}
            type="button"
            aria-current={active ? 'page' : undefined}
            title={name}
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
            <Icon aria-hidden className={cn('size-5', active && hue)} />
            <span className="sr-only md:not-sr-only">{name}</span>
          </button>
        )
      })}
    </nav>
  )
}
