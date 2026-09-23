import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { IssuesPanel } from '@/features/issues/IssuesPanel.tsx'
import { ReviewPanel } from '@/features/review/ReviewPanel.tsx'
import { SettingsPanel } from '@/features/settings/SettingsPanel.tsx'
import { MessagingPanel } from '@/features/messaging/MessagingPanel.tsx'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { EmptyState } from './EmptyState.tsx'
import type { Section } from './uiStore.ts'

// Routes the active section to its panel. The switch is exhaustive over Section,
// so adding a section without a panel is a type error rather than a blank pane.
// Every section but Settings, which reads the configuration rather than the
// stream, waits on the stream's first snapshot; until it lands they share one
// line, rather than each saying it in its own words.
export function SectionPanel({ section }: { section: Section }) {
  const connected = useSnapshotStore((state) => state.snapshot !== null)

  if (!connected && section !== 'settings') {
    return <EmptyState>Connecting to workflow…</EmptyState>
  }

  switch (section) {
    case 'issues':
      return <IssuesPanel />
    case 'branch':
      return <BranchPanel />
    case 'review':
      return <ReviewPanel />
    case 'messaging':
      return <MessagingPanel />
    case 'settings':
      return <SettingsPanel />
  }
}
