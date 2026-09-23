import type { IssueDetail } from '@/api/generated/types.gen.ts'
import { mockSnapshot } from './mockSnapshot.ts'

// mockIssueDetail reads a mock snapshot issue in full for `task web:mockup`:
// the slim issue the list shows, with a believable description, people, a
// comment and a link. Dev-only, and code-split out of a production build.
export function mockIssueDetail(key: string): IssueDetail {
  const issue = mockSnapshot.issues.issues.find((candidate) => candidate.key === key)

  return {
    key,
    summary: issue?.summary ?? key,
    status: issue?.status ?? 'To Do',
    status_category: issue?.status_category ?? 'new',
    type: issue?.type ?? 'Task',
    priority: issue?.priority,
    reporter: 'Ana Lopez',
    assignee: 'ana.lopez',
    description:
      'The request log records every header, so a bearer token lands in the log file.\n\n' +
      'Redact the Authorization header before the line is written.',
    comments: [
      {
        author: 'Sam Ortiz',
        body: "Repro'd on main with --log; the token is in the third line.",
        created: '2026-09-18T15:04:00-06:00',
      },
    ],
    comment_total: 1,
    url: `https://jira.example.com/browse/${key}`,
  }
}
