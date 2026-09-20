import type { Snapshot } from '@/api/generated/types.gen.ts'

// A rich, believable snapshot for `task web:mockup`: enough in every section to
// navigate the whole cockpit without a real Jira, forge, or Slack. Dev-only —
// loaded only when VITE_MOCK is set, and code-split out of a production build.
export const mockSnapshot: Snapshot = {
  issues: {
    total: 4,
    start_at: 0,
    issues: [
      {
        key: 'PROJ-412',
        summary: 'Redact tokens before they reach the request log',
        status: 'In Progress',
        status_category: 'indeterminate',
        type: 'Bug',
        priority: 'High',
      },
      {
        key: 'PROJ-408',
        summary: 'Cache the forge CI status between polls',
        status: 'To Do',
        status_category: 'new',
        type: 'Story',
        priority: 'Medium',
      },
      {
        key: 'PROJ-401',
        summary: 'Slugify the issue summary into the branch name',
        status: 'To Do',
        status_category: 'new',
        type: 'Task',
      },
      {
        key: 'PROJ-377',
        summary: 'Document the on-prem Jira token flow',
        status: 'Done',
        status_category: 'done',
        type: 'Task',
        priority: 'Low',
      },
    ],
  },
  branch: {
    name: 'fix/PROJ-412-redact-tokens',
    detached: false,
    head: 'a1b2c3d',
    upstream: 'origin/fix/PROJ-412-redact-tokens',
    ahead: 3,
    behind: 1,
    base: 'origin/main',
    commits: [
      { hash: 'a1b2c3d4', subject: 'fix: redact tokens in the request log' },
      { hash: 'b2c3d4e5', subject: 'test: prove the log carries no secret' },
      { hash: 'c3d4e5f6', subject: 'refactor: route every write through Redact' },
    ],
  },
  changes: {
    changes: [
      {
        path: 'internal/wiring/reqlog.go',
        kind: 'modified',
        staged: true,
        has_unstaged: false,
        conflicted: false,
      },
      {
        path: 'internal/config/redact.go',
        kind: 'modified',
        staged: false,
        has_unstaged: true,
        conflicted: false,
      },
      {
        path: 'internal/wiring/reqlog_test.go',
        kind: 'new',
        staged: false,
        has_unstaged: true,
        conflicted: false,
      },
    ],
  },
  review: {
    found: true,
    pull: {
      number: 128,
      url: 'https://github.com/acme/workflow/pull/128',
      title: 'fix: redact tokens in the request log',
      draft: false,
      approvals: 1,
      changes_requested: false,
      mergeable: 'clean',
    },
    ci: {
      state: 'running',
      total: 4,
      done: 3,
      failed: 0,
      checks: [
        { name: 'lint', state: 'passed', url: 'https://ci.example.com/lint' },
        { name: 'test', state: 'passed', url: 'https://ci.example.com/test' },
        { name: 'build', state: 'passed', url: 'https://ci.example.com/build' },
        { name: 'e2e', state: 'running', url: 'https://ci.example.com/e2e' },
      ],
    },
  },
  slack: {
    channel: '#dev-workflow',
    channels: ['#dev-workflow', '#releases', '#team-platform'],
    author: 'ana.lopez',
  },
}
