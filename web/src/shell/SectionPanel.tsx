import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { IssuesPanel } from '@/features/issues/IssuesPanel.tsx'
import { ReviewPanel } from '@/features/review/ReviewPanel.tsx'
import { SettingsPanel } from '@/features/settings/SettingsPanel.tsx'
import { SlackPanel } from '@/features/slack/SlackPanel.tsx'
import type { Section } from './uiStore.ts'

// Routes the active section to its panel. The switch is exhaustive over Section,
// so adding a section without a panel is a type error rather than a blank pane.
export function SectionPanel({ section }: { section: Section }) {
  switch (section) {
    case 'issues':
      return <IssuesPanel />
    case 'branch':
      return <BranchPanel />
    case 'review':
      return <ReviewPanel />
    case 'slack':
      return <SlackPanel />
    case 'settings':
      return <SettingsPanel />
  }
}
