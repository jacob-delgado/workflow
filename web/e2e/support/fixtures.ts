import type { Page } from '@playwright/test'
import type { Branch, Issue, IssuesPage, Snapshot } from '../../src/api/generated/types.gen.ts'

// The answers the hermetic specs give the page: the event stream's frame, built
// from one empty frame so each spec names only the parts it varies.

// emptyBranch is a branch with nothing checked out: no name, no head, no
// upstream, nothing ahead or behind.
const emptyBranch = {
  name: '',
  issue_link: '',
  detached: false,
  head: '',
  upstream: '',
  push_remote: '',
  ahead: 0,
  behind: 0,
  base: '',
  commits: [],
} satisfies Branch

// emptySnapshot is the stream's frame of a repository with nothing in it: no
// issues, no branch, no changes, no pull request, and messaging not set up.
const emptySnapshot = {
  here: '/home/ana/src/api',
  issues: { total: 0, start_at: 0, unavailable: [], issues: [] },
  branch: emptyBranch,
  changes: { changes: [] },
  review: { found: false, announced: false },
  messaging: { service: 'Slack', configured: false, channel: '', channels: [], author: '' },
  branches: [],
  commit_types: ['feat', 'fix'],
  subject_limit: 72,
  suggested_scope: '',
  hooks_unmanaged: 0,
  tasks: { available: true, reason: '', linked: [] },
} satisfies Snapshot

// snapshotWith is the empty frame with the parts given in place of its own.
export function snapshotWith(parts: Partial<Snapshot> = {}): Snapshot {
  return { ...emptySnapshot, ...parts }
}

// branchWith is a branch with nothing checked out but the parts given.
export function branchWith(parts: Partial<Branch>): Branch {
  return { ...emptyBranch, ...parts }
}

// issuesOf is the stream's page of issues holding those given, of a view
// holding total in all.
export function issuesOf(issues: Issue[], total = issues.length): IssuesPage {
  return { total, start_at: 0, unavailable: [], issues }
}

// streams answers the event stream with one snapshot.
export async function streams(page: Page, snapshot: Snapshot): Promise<void> {
  await page.route('**/api/events**', (route) =>
    route.fulfill({
      contentType: 'text/event-stream',
      body: `event: snapshot\ndata: ${JSON.stringify(snapshot)}\n\n`,
    }),
  )
}
