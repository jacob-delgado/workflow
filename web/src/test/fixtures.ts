import type {
  Branch,
  Health,
  IssueDetail,
  ReviewRequest,
  Snapshot,
  Stage,
  Task,
  TaskBranch,
  TaskList,
} from '@/api/generated/types.gen.ts'

// A contract-valid branch for tests — the one makeSnapshot checks out, and
// what a write that switches or publishes answers with — with the fields a
// case cares about overridden.
export function makeBranch(overrides: Partial<Branch> = {}): Branch {
  return {
    name: 'fix/PROJ-1',
    issue_link: '',
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

// StageStates is how far each stage of the loop has got, by its step.
type StageStates = Partial<Record<Stage['step'], Stage['state']>>

// makeStages is the loop's five stages as the server sends them, in its order
// and named as it names them, each not started but where states says.
export function makeStages(states: StageStates = {}): Stage[] {
  const stage = (step: Stage['step'], name: string): Stage => ({
    step,
    name,
    state: states[step] ?? 'not_started',
  })

  return [
    stage('issue', 'Issue'),
    stage('branch', 'Branch'),
    stage('commits', 'Commits'),
    stage('review', 'Review'),
    stage('announce', 'Slack'),
  ]
}

// A contract-valid branch named for an issue for tests — fix/PROJ-1, checked
// out, its issue picked up and branched for — with the fields a case cares
// about overridden.
export function makeTaskBranch(overrides: Partial<TaskBranch> = {}): TaskBranch {
  return {
    name: 'fix/PROJ-1',
    issue_key: 'PROJ-1',
    current: true,
    stages: makeStages({ issue: 'done', branch: 'done' }),
    ...overrides,
  }
}

// A complete, contract-valid snapshot for tests, with the fields a case cares
// about overridden. Its stages are those of its branch, which names an issue
// and has nothing committed, and an issue with no branch reads as picked. Kept
// here so every panel test starts from the same shape the stream actually
// pushes.
export function makeSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    here: '/home/ana/src/api',
    issues: { issues: [], total: 0, start_at: 0, unavailable: [] },
    branch: makeBranch(),
    changes: { changes: [] },
    review: { found: false, announced: false },
    stages: makeStages({ issue: 'done', branch: 'done' }),
    unstarted_stages: makeStages({ issue: 'in_flight' }),
    messaging: {
      kind: 'slack',
      service: 'Slack',
      configured: true,
      channel: '#dev',
      channels: [],
      author: 'octocat',
    },
    branches: [],
    commit_types: ['feat', 'fix', 'docs'],
    subject_limit: 72,
    suggested_scope: '',
    hooks_unmanaged: 0,
    tasks: { available: true, reason: '', linked: [] },
    ...overrides,
  }
}

// A contract-valid health answer for tests: a writable server on GitHub, which
// says "pull request", with the fields a case cares about overridden.
export function makeHealth(overrides: Partial<Health> = {}): Health {
  return {
    version: '1.2.3',
    dry_run: false,
    forge_kind: 'github',
    forge_noun: 'pull request',
    forge_sigil: '#',
    ...overrides,
  }
}

// What a server on a GitLab remote says in its health: that it is GitLab, and
// GitLab's own words for a proposed change and the mark before its number.
export const gitLabWords: Partial<Health> = {
  forge_kind: 'gitlab',
  forge_noun: 'merge request',
  forge_sigil: '!',
}

// A contract-valid issue detail for tests — PROJ-1 in Jira, a bug in
// progress with one comment — with the fields a case cares about overridden.
export function makeIssueDetail(overrides: Partial<IssueDetail> = {}): IssueDetail {
  return {
    key: 'PROJ-1',
    tracker: 'jira',
    summary: 'Fix the token leak',
    status: 'In Progress',
    status_category: 'indeterminate',
    type: 'Bug',
    priority: 'High',
    reporter: 'Ana Lopez',
    assignee: 'octocat',
    description: 'Tokens reach the request log.',
    comments: [
      { author: 'Sam Ortiz', body: "Repro'd on main.", created: '2026-09-20T10:00:00-06:00' },
    ],
    comment_total: 1,
    url: 'https://jira.example.com/browse/PROJ-1',
    ...overrides,
  }
}

// A contract-valid Taskwarrior task for tests — task 12, pending and not
// started, tracking PROJ-42 — with the fields a case cares about overridden.
// What the server describes of it — its state, facets, ranks and the fields
// typed text matches — is written out here for this task alone, first in
// every order: a case that changes what the server would describe, and
// reads it, gives its own, as the server would send them.
export function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    uuid: '5f1d7a3c-9b2e-4c8d-a6f0-3e1b2c4d5a6f',
    id: 12,
    description: 'PROJ-42: Redact the token before it reaches the log',
    status: 'pending',
    state: 'pending',
    project: '',
    priority: '',
    tags: [],
    entry: '2026-09-20T10:00:00Z',
    modified: '2026-09-20T10:00:00Z',
    urgency: 4.2,
    annotations: [],
    issue_key: 'PROJ-42',
    issue_url: 'https://jira.example.com/browse/PROJ-42',
    facets: [
      { kind: 'state', value: 'pending', label: 'pending' },
      { kind: 'priority', value: '', label: 'no priority' },
      { kind: 'project', value: '', label: 'no project' },
      { kind: 'issue', value: 'linked', label: 'with issue' },
      { kind: 'tag', value: '', label: 'no tag' },
    ],
    ranks: { urgency: 0, state: 0, id: 0, tag: 0, issue: 0, priority: 0 },
    searchable: ['proj-42: redact the token before it reaches the log', '', 'proj-42', '#12'],
    ...overrides,
  }
}

// A contract-valid task list for tests: Taskwarrior available, holding the
// tasks given, with the fields a case cares about overridden. Its filter
// offers nothing unless the case gives the order the server offers its
// values in.
export function makeTaskList(tasks: Task[], overrides: Partial<TaskList> = {}): TaskList {
  return {
    available: true,
    reason: '',
    context: '',
    sync_available: false,
    said: '',
    tasks,
    facet_order: [],
    ...overrides,
  }
}

// A contract-valid review request for tests — a pull request in acme/api by
// ana, waiting three days, ready, CI failed — with the fields a case cares
// about overridden. Its facets are written out for this request alone, as the
// server labels them: a case that changes what they describe, and reads them,
// gives its own.
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
    facets: [
      { kind: 'repository', value: 'acme/api', label: 'acme/api' },
      { kind: 'ci', value: 'failed', label: 'CI failed' },
      { kind: 'draft', value: 'ready', label: 'ready' },
      { kind: 'author', value: 'ana', label: 'by ana' },
    ],
    ...overrides,
  }
}
