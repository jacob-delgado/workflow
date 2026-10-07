import type { ComponentType } from 'react'
import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { IssuesPanel } from '@/features/issues/IssuesPanel.tsx'
import { ReviewPanel } from '@/features/review/ReviewPanel.tsx'
import { ReviewQueuePanel } from '@/features/reviewqueue/ReviewQueuePanel.tsx'
import { SettingsPanel } from '@/features/settings/SettingsPanel.tsx'
import { MessagingPanel } from '@/features/messaging/MessagingPanel.tsx'
import { RepositoriesPanel } from '@/features/repositories/RepositoriesPanel.tsx'
import { SummaryPanel } from '@/features/summary/SummaryPanel.tsx'
import { TasksPanel } from '@/features/tasks/TasksPanel.tsx'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { StreamWait } from './StreamStatus.tsx'
import type { Section } from './uiStore.ts'

// panels are each section's panel. A record over Section, so adding a section
// without a panel is a type error rather than a blank pane.
const panels: Record<Section, ComponentType> = {
  issues: IssuesPanel,
  branch: BranchPanel,
  review: ReviewPanel,
  messaging: MessagingPanel,
  reviews: ReviewQueuePanel,
  tasks: TasksPanel,
  summary: SummaryPanel,
  repositories: RepositoriesPanel,
  settings: SettingsPanel,
}

// readsOnItsOwn are the sections that do not read the stream: Settings reads
// the configuration, Reviews the forge's queue, Tasks Taskwarrior's list,
// Summary what you did and Repositories where the server works, each on its
// own, so none of them waits on it.
const readsOnItsOwn = new Set<Section>(['settings', 'reviews', 'tasks', 'summary', 'repositories'])

// Routes the active section to its panel. Every section that reads the stream
// waits on its first snapshot; until it lands they share one line, worded by
// the stream's state as the header words it, rather than each saying it in its
// own words.
export function SectionPanel({ section }: { section: Section }) {
  const connected = useSnapshotStore((state) => state.snapshot !== null)

  if (!connected && !readsOnItsOwn.has(section)) {
    return <StreamWait />
  }

  const Panel = panels[section]

  return <Panel />
}
