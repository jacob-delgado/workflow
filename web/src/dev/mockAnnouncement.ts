import type { Announcement, OpenedPullRequest } from '@/api/generated/types.gen.ts'
import { mockConfig } from './mockConfig.ts'
import { mockSnapshot } from './mockSnapshot.ts'

const current = mockSnapshot.branches.find((branch) => branch.current)
const issue = mockSnapshot.issues.issues.find((listed) => listed.key === current?.issue_key)

// The value each placeholder the mock template uses takes, read from the mock
// snapshot as the server reads them from the branch, its issue and its pull.
const values: [string, string][] = [
  ['{author}', mockSnapshot.messaging.author],
  ['{title}', mockSnapshot.review.pull?.title ?? ''],
  ['{url}', mockSnapshot.review.pull?.url ?? ''],
  ['{key}', current?.issue_key ?? ''],
  ['{summary}', issue?.summary ?? ''],
]

// mockAnnouncement is the announcement `task web:mockup` previews and posts:
// the mock configuration's template filled from the mock snapshot, as the
// server fills a Slack template. Dev-only, and code-split out of a production
// build.
export const mockAnnouncement: Announcement = {
  text: values.reduce(
    (text, [placeholder, value]) => text.replaceAll(placeholder, value),
    mockConfig.messaging.announcement ?? '',
  ),
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
