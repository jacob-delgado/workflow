import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { IssuesPanel } from '@/features/issues/IssuesPanel.tsx'
import { EmptyState } from './EmptyState.tsx'
import type { Section } from './uiStore.ts'

// Routes the active section to its panel. Sections without a panel yet show a
// placeholder; each gains its case as it lands.
export function SectionPanel({ section }: { section: Section }) {
  switch (section) {
    case 'issues':
      return <IssuesPanel />
    case 'branch':
      return <BranchPanel />
    default:
      return <EmptyState>This view is coming together.</EmptyState>
  }
}
