import type { Snapshot, Stage } from '@/api/generated/types.gen.ts'
import { mockTasksSummary } from './mockTasks.ts'

// The stages of the checked-out branch's issue: picked up, branched for and
// committed on, its pull request in review while CI runs, and not yet
// announced.
const checkedOutStages: Stage[] = [
  { step: 'issue', name: 'Issue', state: 'done' },
  { step: 'branch', name: 'Branch', state: 'done' },
  { step: 'commits', name: 'Commits', state: 'done' },
  { step: 'review', name: 'Review', state: 'in_flight' },
  { step: 'announce', name: 'Slack', state: 'not_started' },
]

// The stages of an issue on a branch not checked out, as far as the server
// knows them: picked up and branched for, and nothing after begun.
const branchedStages: Stage[] = [
  { step: 'issue', name: 'Issue', state: 'done' },
  { step: 'branch', name: 'Branch', state: 'done' },
  { step: 'commits', name: 'Commits', state: 'not_started' },
  { step: 'review', name: 'Review', state: 'not_started' },
  { step: 'announce', name: 'Slack', state: 'not_started' },
]

// A rich, believable snapshot for `task web:mockup`: enough in every section to
// navigate the whole cockpit without a real Jira, forge, or Slack. Dev-only —
// pushed down the mockup's event stream (mockServer.ts).
export const mockSnapshot: Snapshot = {
  writes: 0,
  here: '/home/ana/src/api',
  issues: {
    total: 7,
    start_at: 0,
    unavailable: [],
    issues: [
      {
        key: 'PROJ-412',
        tracker: 'jira',
        summary: 'Redact tokens in the request log',
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
        status: 'In Review',
        status_category: 'indeterminate',
        type: 'Task',
        priority: 'Low',
      },
    ],
  },
  branch: {
    name: 'fix/PROJ-412-redact-tokens-in-the-request-log',
    issue_link: '',
    detached: false,
    head: '7a91c03',
    upstream: 'origin/fix/PROJ-412-redact-tokens-in-the-request-log',
    push_remote: 'origin',
    base_remote: 'origin',
    ahead: 1,
    behind: 0,
    base: 'origin/main',
    // Newest first: the terminal demo's commit, which it pushes, and a test
    // written since.
    commits: [
      { hash: '7a91c03e', subject: 'test: prove the log carries no secret', unpushed: true },
      {
        hash: '4f2c9e1d',
        subject: 'fix(config): redact tokens in the request log',
        unpushed: false,
      },
    ],
  },
  // Edits made since the test commit, to files the two commits left in
  // place: the morning's new test file is committed, so none is new here.
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
        path: 'internal/config/redact_test.go',
        kind: 'modified',
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
      url: 'https://github.com/acme/api/pull/128',
      title: 'fix(config): redact tokens in the request log',
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
  stages: checkedOutStages,
  // An issue with no branch reads as picked, and nothing after it begun.
  unstarted_stages: [
    { step: 'issue', name: 'Issue', state: 'in_flight' },
    { step: 'branch', name: 'Branch', state: 'not_started' },
    { step: 'commits', name: 'Commits', state: 'not_started' },
    { step: 'review', name: 'Review', state: 'not_started' },
    { step: 'announce', name: 'Slack', state: 'not_started' },
  ],
  messaging: {
    kind: 'slack',
    service: 'Slack',
    configured: true,
    channel: '#dev-workflow',
    channels: ['#dev-workflow', '#releases', '#team-platform'],
    author: 'ana.lopez',
  },
  // Three issues in flight — the checked-out one plus two on other branches — so
  // the mockup shows the issues list marking several, and each with its own story.
  branches: [
    {
      name: 'fix/PROJ-412-redact-tokens-in-the-request-log',
      issue_key: 'PROJ-412',
      current: true,
      stages: checkedOutStages,
    },
    {
      name: 'feat/PROJ-418-webhook-retries',
      issue_key: 'PROJ-418',
      current: false,
      stages: branchedStages,
    },
    {
      name: 'fix/PROJ-408-flaky-timeout',
      issue_key: 'PROJ-408',
      current: false,
      stages: branchedStages,
    },
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
  suggested_scope: 'config',
  hooks_unmanaged: 1,
  // The tasks linked to the checked-out issue and another, one of them started,
  // so the header, the Issues rows and the issue's Tasks card each show one.
  tasks: mockTasksSummary,
}
