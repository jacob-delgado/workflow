import type { Snapshot } from '@/api/generated/types.gen.ts'
import { mockTasksSummary } from './mockTasks.ts'

// A rich, believable snapshot for `task web:mockup`: enough in every section to
// navigate the whole cockpit without a real Jira, forge, or Slack. Dev-only —
// loaded only when VITE_MOCK is set, and code-split out of a production build.
export const mockSnapshot: Snapshot = {
  here: '/home/ana/src/api',
  issues: {
    total: 7,
    start_at: 0,
    unavailable: [],
    issues: [
      {
        key: 'PROJ-412',
        tracker: 'jira',
        summary: 'Redact tokens before they reach the request log',
        status: 'In Progress',
        status_category: 'indeterminate',
        type: 'Bug',
        priority: 'High',
      },
      {
        key: 'PROJ-418',
        tracker: 'jira',
        summary: 'Refuse to start when the config names an unknown forge',
        status: 'In Review',
        status_category: 'indeterminate',
        type: 'Bug',
        priority: 'High',
      },
      {
        key: 'PROJ-408',
        tracker: 'jira',
        summary: 'Cache the forge CI status between polls',
        status: 'To Do',
        status_category: 'new',
        type: 'Story',
        priority: 'Medium',
      },
      {
        key: 'PROJ-401',
        tracker: 'jira',
        summary: 'Slugify the issue summary into the branch name',
        status: 'To Do',
        status_category: 'new',
        type: 'Task',
      },
      {
        key: 'PROJ-396',
        tracker: 'jira',
        summary: 'Support GitLab merge requests alongside GitHub pulls',
        status: 'Backlog',
        status_category: 'new',
        type: 'Story',
        priority: 'Medium',
      },
      {
        key: 'PROJ-390',
        tracker: 'jira',
        summary: 'Flake in the CI-polling test under the race detector',
        status: 'In Progress',
        status_category: 'indeterminate',
        type: 'Bug',
        priority: 'Low',
      },
      {
        key: 'PROJ-377',
        tracker: 'jira',
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
    issue_link: '',
    detached: false,
    head: 'a1b2c3d',
    upstream: 'origin/fix/PROJ-412-redact-tokens',
    push_remote: 'origin',
    ahead: 3,
    behind: 1,
    base: 'origin/main',
    commits: [
      { hash: 'a1b2c3d4', subject: 'fix: redact tokens in the request log', unpushed: true },
      { hash: 'b2c3d4e5', subject: 'test: prove the log carries no secret', unpushed: true },
      { hash: 'c3d4e5f6', subject: 'refactor: route every write through Redact', unpushed: true },
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
        path: 'internal/sanitize/mask.go',
        original_path: 'internal/config/mask.go',
        kind: 'renamed',
        staged: true,
        has_unstaged: false,
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
    announced: false,
    pull: {
      number: 128,
      url: 'https://github.com/acme/workflow/pull/128',
      title: 'fix: redact tokens in the request log',
      state: 'open',
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
  messaging: {
    service: 'Slack',
    configured: true,
    channel: '#dev-workflow',
    channels: ['#dev-workflow', '#releases', '#team-platform'],
    author: 'ana.lopez',
  },
  // Three issues in flight — the checked-out one plus two on other branches — so
  // the mockup shows the issues list marking several, and each with its own story.
  branches: [
    { name: 'fix/PROJ-412-redact-tokens', issue_key: 'PROJ-412', current: true },
    { name: 'feat/PROJ-418-webhook-retries', issue_key: 'PROJ-418', current: false },
    { name: 'fix/PROJ-408-flaky-timeout', issue_key: 'PROJ-408', current: false },
  ],
  // The built-in Conventional Commit types, as a server whose configuration
  // names none sends them.
  commit_types: [
    'feat',
    'fix',
    'docs',
    'refactor',
    'test',
    'perf',
    'build',
    'ci',
    'chore',
    'style',
    'revert',
  ],
  subject_limit: 72,
  // The scope the last commit here used, so the mockup's commit form opens on it.
  suggested_scope: 'wiring',
  hooks_unmanaged: 1,
  // The tasks linked to the checked-out issue and another, one of them started,
  // so the header, the Issues rows and the issue's Tasks card each show one.
  tasks: mockTasksSummary,
}
