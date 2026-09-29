import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { IssuesPanel } from '@/features/issues/IssuesPanel.tsx'
import { ReviewPanel } from '@/features/review/ReviewPanel.tsx'
import { ReviewQueuePanel } from '@/features/reviewqueue/ReviewQueuePanel.tsx'
import { SettingsPanel } from '@/features/settings/SettingsPanel.tsx'
import { MessagingPanel } from '@/features/messaging/MessagingPanel.tsx'
import { TasksPanel } from '@/features/tasks/TasksPanel.tsx'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { EmptyState } from './EmptyState.tsx'
import type { Section } from './uiStore.ts'

// Routes the active section to its panel. The switch is exhaustive over Section,
// so adding a section without a panel is a type error rather than a blank pane.
// Every section that reads the stream waits on its first snapshot; until it
// lands they share one line, rather than each saying it in its own words.
export function SectionPanel({ section }: { section: Section }) {
  const connected = useSnapshotStore((state) => state.snapshot !== null)

  if (!connected && waitsOnStream(section)) {
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
    case 'tasks':
      return <TasksPanel />
    case 'settings':
      return <SettingsPanel />
  }
}

// waitsOnStream reports a section that reads the stream, so waits on its first
// snapshot. Settings reads the configuration, Reviews the forge's queue and
// Tasks Taskwarrior's list, each on its own, so none of them waits.
function waitsOnStream(section: Section): boolean {
  return section !== 'settings' && section !== 'reviews' && section !== 'tasks'
}
