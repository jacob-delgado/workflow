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

// The label, rail icon and hue of each section — one source, read by the nav
// rail and by the content area's heading. The hue is the system the section
// belongs to, as the terminal's spine colors its stages (internal/tui/spine.go):
// Jira's issues, git's branch, the forge's pull requests, the messaging
// service. Settings belongs to none of them, so it stays in the ink.
export const sectionMeta: Record<Section, { label: string; Icon: LucideIcon; hue: string }> = {
  issues: { label: 'Issues', Icon: CircleDot, hue: 'text-jira' },
  branch: { label: 'Branch', Icon: GitBranch, hue: 'text-git' },
  review: { label: 'Review', Icon: GitPullRequest, hue: 'text-forge' },
  messaging: { label: 'Messaging', Icon: Send, hue: 'text-messaging' },
  reviews: { label: 'Reviews', Icon: Inbox, hue: 'text-forge' },
  settings: { label: 'Settings', Icon: Settings, hue: 'text-foreground' },
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
