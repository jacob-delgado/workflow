import { NavRail } from './NavRail.tsx'
import { sectionMeta } from './sections.ts'
import { useUiStore } from './uiStore.ts'

export function AppShell() {
  const section = useUiStore((state) => state.section)
  const { label } = sectionMeta[section]

  return (
    <div className="flex min-h-dvh">
      <NavRail />
      <main className="flex-1 overflow-auto px-6 py-5">
        <h1 className="text-2xl font-semibold tracking-tight">{label}</h1>
        <p className="mt-2 text-muted-foreground">This section is coming together.</p>
      </main>
    </div>
  )
}
