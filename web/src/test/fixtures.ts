import type {
  Branch,
  Health,
  ReviewRequest,
  Snapshot,
  Task,
  TaskList,
} from '@/api/generated/types.gen.ts'

// A contract-valid branch for tests — the one makeSnapshot checks out, and
// what a write that switches or publishes answers with — with the fields a
// case cares about overridden.
export function makeBranch(overrides: Partial<Branch> = {}): Branch {
  return {
    name: 'fix/PROJ-1',
    detached: false,
    head: 'abc1234',
    upstream: 'origin/fix/PROJ-1',
    push_remote: 'origin',
    ahead: 2,
    behind: 0,
    base: 'origin/main',
    commits: [],
    ...overrides,
  }
}

// A complete, contract-valid snapshot for tests, with the fields a case cares
// about overridden. Kept here so every panel test starts from the same shape
// the stream actually pushes.
export function makeSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    issues: { issues: [], total: 0, start_at: 0, unavailable: [] },
    branch: makeBranch(),
    changes: { changes: [] },
    review: { found: false },
    messaging: {
      service: 'Slack',
      configured: true,
      channel: '#dev',
      channels: [],
      author: 'octocat',
    },
    branches: [],
    commit_types: ['feat', 'fix', 'docs'],
    suggested_scope: '',
    tasks: { available: true, reason: '', linked: [] },
    ...overrides,
  }
}

// A contract-valid health answer for tests: a writable server on a forge that
// says "pull request", with the fields a case cares about overridden.
export function makeHealth(overrides: Partial<Health> = {}): Health {
  return {
    version: '1.2.3',
    dry_run: false,
    forge_noun: 'pull request',
    forge_sigil: '#',
    ...overrides,
  }
}

// What a server on a GitLab remote says in its health: GitLab's own words for a
// proposed change and the mark before its number.
export const gitLabWords: Partial<Health> = { forge_noun: 'merge request', forge_sigil: '!' }

// A contract-valid Taskwarrior task for tests — task 12, pending and not
// started, tracking PROJ-42 — with the fields a case cares about overridden.
export function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    uuid: '5f1d7a3c-9b2e-4c8d-a6f0-3e1b2c4d5a6f',
    id: 12,
    description: 'PROJ-42: Redact the token before it reaches the log',
    status: 'pending',
    project: '',
    priority: '',
    tags: [],
    entry: '2026-09-20T10:00:00Z',
    modified: '2026-09-20T10:00:00Z',
    urgency: 4.2,
    annotations: [],
    issue_key: 'PROJ-42',
    issue_url: 'https://jira.example.com/browse/PROJ-42',
    ...overrides,
  }
}

// A contract-valid task list for tests: an available Taskwarrior with no
// context and no sync backend, listing the tasks given.
export function makeTaskList(tasks: Task[], overrides: Partial<TaskList> = {}): TaskList {
  return {
    available: true,
    reason: '',
    context: '',
    sync_available: false,
    said: '',
    tasks,
    ...overrides,
  }
}

// A contract-valid review request for tests — a pull request waiting three
// days, CI failed — with the fields a case cares about overridden.
export function makeReviewRequest(overrides: Partial<ReviewRequest> = {}): ReviewRequest {
  return {
    number: 42,
    url: 'https://github.com/acme/api/pull/42',
    title: 'fix: redact the token before it reaches the log',
    author: 'ana',
    repository: 'acme/api',
    draft: false,
    ci: 'failed',
    opened_at: new Date(Date.now() - (3 * 24 + 1) * 3_600_000).toISOString(),
    ...overrides,
  }
}
