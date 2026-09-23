import {
  CircleDot,
  GitBranch,
  GitPullRequest,
  Inbox,
  Send,
  Settings,
  type LucideIcon,
} from 'lucide-react'
import type { Section } from './uiStore.ts'

// The label and rail icon for each section — one source, read by the nav rail
// and by the content area's heading, both through sectionLabel.
export const sectionMeta: Record<Section, { label: string; Icon: LucideIcon }> = {
  issues: { label: 'Issues', Icon: CircleDot },
  branch: { label: 'Branch', Icon: GitBranch },
  review: { label: 'Review', Icon: GitPullRequest },
  messaging: { label: 'Messaging', Icon: Send },
  reviews: { label: 'Reviews', Icon: Inbox },
  settings: { label: 'Settings', Icon: Settings },
}

// sectionLabel names a section as the nav rail and the content heading show it.
// The messaging section takes the configured service's name — Slack, Teams — as
// the interface's pane title does, once the stream has said which service it is.
export function sectionLabel(section: Section, service: string | undefined): string {
  if (section === 'messaging' && service !== undefined && service !== '') {
    return service
  }

  return sectionMeta[section].label
}
