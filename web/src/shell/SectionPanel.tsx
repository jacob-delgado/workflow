import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { IssuesPanel } from '@/features/issues/IssuesPanel.tsx'
import { ReviewPanel } from '@/features/review/ReviewPanel.tsx'
import { ReviewQueuePanel } from '@/features/reviewqueue/ReviewQueuePanel.tsx'
import { SettingsPanel } from '@/features/settings/SettingsPanel.tsx'
import { MessagingPanel } from '@/features/messaging/MessagingPanel.tsx'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { EmptyState } from './EmptyState.tsx'
import type { Section } from './uiStore.ts'

// Routes the active section to its panel. The switch is exhaustive over Section,
// so adding a section without a panel is a type error rather than a blank pane.
// Every section that reads the stream waits on its first snapshot; until it
// lands they share one line, rather than each saying it in its own words.
// Settings reads the configuration and Reviews the forge's queue, each on its
// own, so neither waits.
export function SectionPanel({ section }: { section: Section }) {
  const connected = useSnapshotStore((state) => state.snapshot !== null)

  if (!connected && section !== 'settings' && section !== 'reviews') {
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
    case 'reviews':
      return <ReviewQueuePanel />
    case 'settings':
      return <SettingsPanel />
  }
}
