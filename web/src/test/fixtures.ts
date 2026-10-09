import type {
  Branch,
  Health,
  IssueDetail,
  ReviewFacet,
  ReviewRequest,
  Snapshot,
  Stage,
  Task,
  TaskBranch,
  TaskFacet,
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

// TaskDescription is what the server describes of a task, rather than what
// Taskwarrior holds of it: where the task stands, the values it holds, labeled
// as the server labels them and in the order it gives them, and the fields
// typed text matches, lower-cased. A case writes it out, as the server would
// send it; the page works none of it out.
export type TaskDescription = Pick<Task, 'state' | 'facets' | 'searchable'>

// TaskFields are what Taskwarrior holds of a task, with its issue's page and
// its place in each order of the list that holds it.
type TaskFields = Omit<Task, keyof TaskDescription>

// DescribingFields are the fields the server reads a task's description from.
// A task that differs from task 12 in any of them is made by describedTask,
// with them and its description given together.
type DescribingFields =
  'id' | 'description' | 'status' | 'start' | 'wait' | 'project' | 'priority' | 'tags' | 'issue_key'

// The values the cases' tasks hold most often, each as the server labels it.
export const taskFacet = {
  started: { kind: 'state', value: 'started', label: 'started' },
  pending: { kind: 'state', value: 'pending', label: 'pending' },
  waiting: { kind: 'state', value: 'waiting', label: 'waiting' },
  recurring: { kind: 'state', value: 'recurring', label: 'recurring' },
  completed: { kind: 'state', value: 'completed', label: 'completed' },
  deleted: { kind: 'state', value: 'deleted', label: 'deleted' },
  noPriority: { kind: 'priority', value: '', label: 'no priority' },
  noProject: { kind: 'project', value: '', label: 'no project' },
  withIssue: { kind: 'issue', value: 'linked', label: 'with issue' },
  noIssue: { kind: 'issue', value: 'unlinked', label: 'no issue' },
  noTag: { kind: 'tag', value: '', label: 'no tag' },
} as const satisfies Record<string, TaskFacet>

// Standing is where the server reads a task to stand, and the value its state
// facet holds for it.
interface Standing {
  state: Task['state']
  facet: TaskFacet
}

// The places the cases' tasks stand, each as the server gives it.
export const standing = {
  started: { state: 'started', facet: taskFacet.started },
  pending: { state: 'pending', facet: taskFacet.pending },
  waiting: { state: 'waiting', facet: taskFacet.waiting },
  recurring: { state: 'recurring', facet: taskFacet.recurring },
  completed: { state: 'completed', facet: taskFacet.completed },
  deleted: { state: 'deleted', facet: taskFacet.deleted },
} as const satisfies Record<string, Standing>

// task12 is a contract-valid Taskwarrior task for tests — task 12, pending and
// not started, tracking PROJ-42 — as the server answers it, the only task of
// its list, so first in every order.
const task12: Task = {
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
    taskFacet.pending,
    taskFacet.noPriority,
    taskFacet.noProject,
    taskFacet.withIssue,
    taskFacet.noTag,
  ],
  ranks: { urgency: 0, state: 0, id: 0, tag: 0, issue: 0, priority: 0 },
  searchable: ['proj-42: redact the token before it reaches the log', '', 'proj-42', '#12'],
}

// makeTask is task 12, with the fields a case cares about that its description
// is not read from overridden: its uuid, dates, urgency, notes, issue page and
// ranks.
export function makeTask(overrides: Partial<Omit<TaskFields, DescribingFields>> = {}): Task {
  return { ...task12, ...overrides }
}

// describedTask is a task as the server answers it: every field its
// description is read from — a start or a wait left out is none — and its
// issue's page, given together with that description, and task 12's for the
// rest unless fields says otherwise.
export function describedTask(
  fields: Pick<TaskFields, Exclude<DescribingFields, 'start' | 'wait'> | 'issue_url'> &
    Partial<TaskFields>,
  description: TaskDescription,
): Task {
  return {
    ...task12,
    ...fields,
    state: description.state,
    facets: description.facets,
    searchable: description.searchable,
  }
}

// taskStanding is task as the server answers it once Taskwarrior holds the
// fields given for it — a status, a start, a wait or an end — and it reads the
// task to stand as standing says, its state's facet that one, wherever the
// server put it. The rest of what the server describes of the task stays as
// it was.
export function taskStanding(
  task: Task,
  fields: Partial<Pick<TaskFields, 'status' | 'start' | 'wait' | 'end'>>,
  { state, facet }: Standing,
): Task {
  const facets = task.facets.map((held) => (held.kind === 'state' ? facet : held))

  return describedTask({ ...task, ...fields }, { state, facets, searchable: task.searchable })
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

// ReviewFields are what the forge says of a review request, without the
// facets the server describes it by.
type ReviewFields = Omit<ReviewRequest, 'facets'>

// FacetedFields are the fields the server reads a review request's facets
// from. A request that differs from request 42 in any of them is made by
// describedReviewRequest, with them and its facets given together.
type FacetedFields = 'repository' | 'ci' | 'draft' | 'author'

// request42 is a contract-valid review request for tests — a pull request in
// acme/api by ana, waiting three days, ready, CI failed — with its facets as
// the server labels them.
function request42(): ReviewRequest {
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
  }
}

// makeReviewRequest is request 42, with the fields a case cares about that its
// facets are not read from overridden.
export function makeReviewRequest(
  overrides: Partial<Omit<ReviewFields, FacetedFields>> = {},
): ReviewRequest {
  return { ...request42(), ...overrides }
}

// describedReviewRequest is a review request as the server answers it: every
// field its facets are read from, given together with those facets, and
// request 42's for the rest unless fields says otherwise.
export function describedReviewRequest(
  fields: Pick<ReviewFields, FacetedFields> & Partial<ReviewFields>,
  facets: ReviewFacet[],
): ReviewRequest {
  return { ...request42(), ...fields, facets }
}
