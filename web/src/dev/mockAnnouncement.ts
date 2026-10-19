import type { Announcement, OpenedPullRequest } from '@/api/generated/types.gen.ts'
import { mockConfig } from './mockConfig.ts'
import { mockSnapshot } from './mockSnapshot.ts'

const current = mockSnapshot.branches.find((branch) => branch.current)
const issue = mockSnapshot.issues.issues.find((listed) => listed.key === current?.issue_key)
const pull = mockSnapshot.review.pull

// builtInText is the Slack announcement the server composes when the
// configuration names no template, as the mock's does not: the lead sentence
// linking the pull request, then the issue linked to its page and its summary
// (internal/messaging's defaultText). Every value is the mock's own, with
// nothing in it for Slack to read as markup.
function builtInText(): string {
  const key = current?.issue_key ?? ''
  const issueURL = `${mockConfig.jira.base_url ?? ''}/browse/${key}`
  const lead = `${mockSnapshot.messaging.author} opened a pull request: <${pull?.url ?? ''}|${pull?.title ?? ''}>`

  return `${lead}\n<${issueURL}|${key}> ${issue?.summary ?? ''}`
}

// mockAnnouncement is the announcement `task web:mockup` previews and posts:
// the built-in message for the mock pull request, as the server composes it
// from the branch, its issue and its pull. Dev-only, and code-split out of a
// production build.
export const mockAnnouncement: Announcement = {
  text: builtInText(),
  channel: mockSnapshot.messaging.channel,
  ci: mockSnapshot.review.ci?.state,
  // The mock pull request's CI has not passed, so the preview offers to wait.
  can_wait_for_ci: mockSnapshot.review.ci?.state !== 'passed',
}

// mockArmedOn is what opening pull request number says of its announcement:
// armed to post once CI passes, or, for a draft, why it is not.
export function mockArmedOn(
  number: number,
  draft: boolean,
): Pick<OpenedPullRequest, 'announcement' | 'not_armed'> {
  return draft
    ? { not_armed: 'a draft is not announced; announce it once it is ready' }
    : {
        announcement: {
          state: 'waiting',
          channel: mockSnapshot.messaging.channel,
          pull: number,
          automatic: true,
        },
      }
}
