import type { IssueDetail, StatusChange, ViewList } from '@/api/generated/types.gen.ts'
import { mockSnapshot } from './mockSnapshot.ts'

// mockIssueDetail reads a mock snapshot issue in full for `task web:mockup`:
// the slim issue the list shows, with a believable description, people, a
// comment and a link. Dev-only, and code-split out of a production build.
export function mockIssueDetail(key: string): IssueDetail {
  const issue = mockSnapshot.issues.issues.find((candidate) => candidate.key === key)

  return {
    key,
    tracker: 'jira',
    summary: issue?.summary ?? key,
    status: issue?.status ?? 'To Do',
    status_category: issue?.status_category ?? 'new',
    type: issue?.type ?? 'Task',
    priority: issue?.priority,
    reporter: 'Ana Lopez',
    assignee: 'Ana Lopez',
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

// mockViews are the saved views the mockup's view select offers.
export const mockViews: ViewList = {
  views: [
    { name: 'Assigned to me', jql: 'assignee = currentUser() AND resolution = Unresolved' },
    { name: 'Team bugs', jql: 'project = PROJ AND type = Bug AND resolution = Unresolved' },
  ],
}

// mockStatusChanges are the status changes the mockup's issues offer: one
// that needs nothing, one with a field form, and one only Jira can make.
export const mockStatusChanges: StatusChange[] = [
  {
    id: '11',
    name: 'Block',
    to_status: 'Blocked',
    to_status_category: 'indeterminate',
    fields: [],
  },
  {
    id: '21',
    name: 'Resolve Issue',
    to_status: 'Resolved',
    to_status_category: 'done',
    fields: [
      { id: 'duedate', name: 'Due date', kind: 'date', options: [] },
      {
        id: 'fixVersions',
        name: 'Fix versions',
        kind: 'option_list',
        options: [
          { id: '10', name: '1.4.0' },
          { id: '11', name: '1.5.0' },
        ],
      },
      {
        id: 'resolution',
        name: 'Resolution',
        kind: 'option',
        options: [
          { id: '1', name: 'Fixed' },
          { id: '2', name: "Won't Fix" },
        ],
      },
    ],
  },
  {
    id: '31',
    name: 'Split',
    to_status: 'Split',
    to_status_category: 'done',
    fields: [{ id: 'customfield_2', name: 'Component tree', kind: 'only_jira', options: [] }],
  },
]
