import type {
  Branch,
  Health,
  ReviewFacet,
  ReviewRequest,
  Snapshot,
  Stage,
  Task,
  TaskFacet,
  TaskList,
  TaskRanks,
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
  const stage = (step: Stage['step'], name: string, system: Stage['system']): Stage => ({
    step,
    name,
    system,
    state: states[step] ?? 'not_started',
  })

  return [
    stage('issue', 'Issue', 'tracker'),
    stage('branch', 'Branch', 'git'),
    stage('commits', 'Commits', 'git'),
    stage('review', 'Review', 'forge'),
    stage('announce', 'Slack', 'messaging'),
  ]
}

// A complete, contract-valid snapshot for tests, with the fields a case cares
// about overridden. Its stages are those of its branch, which names an issue
// and has nothing committed. Kept here so every panel test starts from the same shape
// the stream actually pushes.
export function makeSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    here: '/home/ana/src/api',
    issues: { issues: [], total: 0, start_at: 0, unavailable: [] },
    branch: makeBranch(),
    changes: { changes: [] },
    review: { found: false, announced: false },
    stages: makeStages({ issue: 'done', branch: 'done' }),
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

// A contract-valid Taskwarrior task for tests — task 12, pending and not
// started, tracking PROJ-42 — with the fields a case cares about overridden.
// What the server describes of a task — where it stands, its facets and the
// fields typed text matches — follows from the case's fields unless the case
// gives its own; its ranks are first in every order unless the case gives
// them, so a case that sorts says where each task goes.
export function makeTask(overrides: Partial<Task> = {}): Task {
  const task = {
    uuid: '5f1d7a3c-9b2e-4c8d-a6f0-3e1b2c4d5a6f',
    id: 12,
    description: 'PROJ-42: Redact the token before it reaches the log',
    status: 'pending' as const,
    project: '',
    priority: '',
    tags: [] as string[],
    entry: '2026-09-20T10:00:00Z',
    modified: '2026-09-20T10:00:00Z',
    urgency: 4.2,
    annotations: [],
    issue_key: 'PROJ-42',
    issue_url: 'https://jira.example.com/browse/PROJ-42',
    ...overrides,
  }
  const state = overrides.state ?? (task.start === undefined ? task.status : 'started')

  return {
    ...task,
    state,
    facets: overrides.facets ?? taskFacetsOf({ ...task, state }),
    ranks: overrides.ranks ?? firstInEveryOrder,
    searchable: overrides.searchable ?? searchableOf(task),
  }
}

// firstInEveryOrder is a task's ranks when a case does not sort.
const firstInEveryOrder: TaskRanks = { urgency: 0, state: 0, id: 0, tag: 0, issue: 0, priority: 0 }

// taskFacetsOf is what the server ships as a task's facets, for the case data
// a test gives: each value as the server labels it, written out for the data.
function taskFacetsOf(
  task: Pick<Task, 'state' | 'priority' | 'project' | 'issue_key' | 'tags'>,
): TaskFacet[] {
  const named = (kind: TaskFacet['kind'], value: string, label: string, none: string) => ({
    kind,
    value,
    label: value === '' ? none : label,
  })
  const tags = task.tags.length === 0 ? [''] : task.tags

  return [
    named('state', task.state, task.state, 'no state'),
    named('priority', task.priority, `priority ${task.priority}`, 'no priority'),
    named('project', task.project, `project ${task.project}`, 'no project'),
    task.issue_key === ''
      ? { kind: 'issue', value: 'unlinked', label: 'no issue' }
      : { kind: 'issue', value: 'linked', label: 'with issue' },
    ...tags.map((tag) => named('tag', tag, `+${tag}`, 'no tag')),
  ]
}

// searchableOf is the fields the server ships for typed text to match, for
// the case data a test gives.
function searchableOf(task: Pick<Task, 'description' | 'project' | 'issue_key' | 'tags' | 'id'>) {
  const fields = [
    task.description,
    task.project,
    task.issue_key,
    ...task.tags.map((tag) => `+${tag}`),
  ]
  if (task.id > 0) {
    fields.push(`#${String(task.id)}`)
  }

  return fields.map((field) => field.toLowerCase())
}

export function makeTaskList(tasks: Task[], overrides: Partial<TaskList> = {}): TaskList {
  return {
    available: true,
    reason: '',
    context: '',
    sync_available: false,
    said: '',
    tasks,
    facet_order: offeredOf(tasks),
    ...overrides,
  }
}

// offeredOf stands in for the order the server offers the tasks' values in:
// each value they hold, once, in the order the tasks hold them. A case about
// the order the filter offers its values in gives the server's own.
function offeredOf(tasks: Task[]): TaskFacet[] {
  const offered: TaskFacet[] = []
  for (const facet of tasks.flatMap((task) => task.facets)) {
    if (!offered.some((one) => one.kind === facet.kind && one.value === facet.value)) {
      offered.push(facet)
    }
  }

  return offered
}

// A contract-valid review request for tests — a pull request waiting three
// days, CI failed — with the fields a case cares about overridden.
export function makeReviewRequest(overrides: Partial<ReviewRequest> = {}): ReviewRequest {
  const request = {
    number: 42,
    url: 'https://github.com/acme/api/pull/42',
    title: 'fix: redact the token before it reaches the log',
    author: 'ana',
    repository: 'acme/api',
    draft: false,
    ci: 'failed' as const,
    opened_at: new Date(Date.now() - (3 * 24 + 1) * 3_600_000).toISOString(),
    ...overrides,
  }

  return { ...request, facets: overrides.facets ?? reviewFacetsOf(request) }
}

// reviewFacetsOf is what the server ships as a request's facets, for the case
// data a test gives: each value as the server labels it, here written out for
// the data alone.
function reviewFacetsOf(
  request: Pick<ReviewRequest, 'repository' | 'ci' | 'draft' | 'author'>,
): ReviewFacet[] {
  const readiness = request.draft ? 'draft' : 'ready'

  return [
    {
      kind: 'repository',
      value: request.repository,
      label: request.repository === '' ? 'no repository' : request.repository,
    },
    { kind: 'ci', value: request.ci, label: `CI ${request.ci}` },
    { kind: 'draft', value: readiness, label: readiness },
    { kind: 'author', value: request.author, label: `by ${request.author}` },
  ]
}
