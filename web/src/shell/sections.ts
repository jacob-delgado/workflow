import { CircleDot, GitBranch, GitPullRequest, Send, Settings, type LucideIcon } from 'lucide-react'
import type { Section } from './uiStore.ts'

// The label and rail icon for each section — one source, read by the nav rail
// and by the content area's heading.
export const sectionMeta: Record<Section, { label: string; Icon: LucideIcon }> = {
  issues: { label: 'Issues', Icon: CircleDot },
  branch: { label: 'Branch', Icon: GitBranch },
  review: { label: 'Review', Icon: GitPullRequest },
  messaging: { label: 'Messaging', Icon: Send },
  settings: { label: 'Settings', Icon: Settings },
}
