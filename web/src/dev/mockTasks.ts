import type {
  Task,
  TaskFacet,
  TaskList,
  TaskRanks,
  TasksSummary,
} from '@/api/generated/types.gen.ts'
import { minute } from '@/lib/dates.ts'

// at is the RFC 3339 time some minutes from now — before it when negative — so
// the mockup's ages and due dates read the same whenever it is shown.
function at(minutes: number): string {
  return new Date(Date.now() + minutes * minute).toISOString()
}

const hours = 60
const days = 24 * hours

// facet is a value a task holds, as the server labels it.
function facet(kind: TaskFacet['kind'], value: string, label: string): TaskFacet {
  return { kind, value, label }
}

// The values the mockup's tasks hold, as the server describes them.
const isStarted = facet('state', 'started', 'started')
const isPending = facet('state', 'pending', 'pending')
const isWaiting = facet('state', 'waiting', 'waiting')
const isCompleted = facet('state', 'completed', 'completed')
const atHigh = facet('priority', 'H', 'priority H')
const atMedium = facet('priority', 'M', 'priority M')
const atNone = facet('priority', '', 'no priority')
const inWorkflow = facet('project', 'workflow', 'project workflow')
const inOps = facet('project', 'ops', 'project ops')
const inNoProject = facet('project', '', 'no project')
const linked = facet('issue', 'linked', 'with issue')
const unlinked = facet('issue', 'unlinked', 'no issue')
const taggedJira = facet('tag', 'jira', '+jira')
const untagged = facet('tag', '', 'no tag')

// redacting is the started task, tracking the checked-out issue as tracking
// it writes one — its key and page, the jira tag, its priority and a note of
// the page — with its description since shortened by a modify.
const redacting: Task = {
  uuid: '7c1e9a42-3b5d-4f60-8a1c-2d4e6f80a1b3',
  id: 1,
  description: 'PROJ-412: Fix token redaction',
  status: 'pending',
  project: 'workflow',
  priority: 'H',
  tags: ['jira'],
  start: at(-72),
  entry: at(-3 * days),
  modified: at(-72),
  urgency: 14.2,
  annotations: [
    { entry: at(-3 * days), description: 'https://jira.example.com/browse/PROJ-412' },
    { entry: at(-26 * hours), description: 'Ana can review it once CI is green' },
  ],
  issue_key: 'PROJ-412',
  issue_url: 'https://jira.example.com/browse/PROJ-412',
  state: 'started',
  facets: [isStarted, atHigh, inWorkflow, linked, taggedJira],
  ranks: { urgency: 0, state: 0, id: 0, tag: 0, issue: 1, priority: 0 },
  searchable: ['proj-412: fix token redaction', 'workflow', 'proj-412', '+jira', '#1'],
}

// caching is a task still to do for another issue of yours, due in two days.
const caching: Task = {
  uuid: '2f4a6c8e-0b1d-4e3f-9a5b-7c9d1e3f5a7b',
  id: 2,
  description: 'PROJ-408: Cache the forge CI status between polls',
  status: 'pending',
  project: 'workflow',
  priority: 'M',
  tags: ['jira'],
  due: at(2 * days),
  entry: at(-5 * days),
  modified: at(-5 * days),
  urgency: 9.1,
  annotations: [{ entry: at(-5 * days), description: 'https://jira.example.com/browse/PROJ-408' }],
  issue_key: 'PROJ-408',
  issue_url: 'https://jira.example.com/browse/PROJ-408',
  state: 'pending',
  facets: [isPending, atMedium, inWorkflow, linked, taggedJira],
  ranks: { urgency: 1, state: 1, id: 1, tag: 1, issue: 0, priority: 1 },
  searchable: [
    'proj-408: cache the forge ci status between polls',
    'workflow',
    'proj-408',
    '+jira',
    '#2',
  ],
}

// certificate is a task no issue tracks, due tomorrow.
const certificate: Task = {
  uuid: '9e8d7c6b-5a4f-4e3d-8c2b-1a0f9e8d7c6b',
  id: 3,
  description: 'Renew the staging TLS certificate',
  status: 'pending',
  project: 'ops',
  priority: '',
  tags: [],
  due: at(1 * days),
  entry: at(-2 * days),
  modified: at(-2 * days),
  urgency: 6.3,
  annotations: [],
  issue_key: '',
  issue_url: '',
  state: 'pending',
  facets: [isPending, atNone, inOps, unlinked, untagged],
  ranks: { urgency: 2, state: 2, id: 2, tag: 2, issue: 2, priority: 2 },
  searchable: ['renew the staging tls certificate', 'ops', '', '#3'],
}

// retro is a task no issue tracks, hidden until later in the week.
const retro: Task = {
  uuid: '4b3a2918-0f7e-4d6c-9b5a-483726150f4e',
  id: 4,
  description: 'Book a room for the sprint retro',
  status: 'waiting',
  project: '',
  priority: '',
  tags: [],
  wait: at(3 * days),
  entry: at(-1 * days),
  modified: at(-1 * days),
  urgency: 1.8,
  annotations: [],
  issue_key: '',
  issue_url: '',
  state: 'waiting',
  facets: [isWaiting, atNone, inNoProject, unlinked, untagged],
  ranks: { urgency: 3, state: 3, id: 3, tag: 3, issue: 3, priority: 3 },
  searchable: ['book a room for the sprint retro', '', '', '#4'],
}

// mockTaskList is the task list `task web:mockup` shows: a started task for the
// checked-out issue, one for another issue, one no issue tracks and one
// waiting, most urgent first, with a sync backend to offer Sync. Dev-only, and
// code-split out of a production build.
function mockTaskList(): TaskList {
  return {
    available: true,
    reason: '',
    context: '',
    sync_available: true,
    said: '',
    tasks: [redacting, caching, certificate, retro],
    facet_order: [
      isStarted,
      isPending,
      isWaiting,
      facet('state', 'recurring', 'recurring'),
      isCompleted,
      facet('state', 'deleted', 'deleted'),
      facet('state', 'unknown', 'unknown'),
      atHigh,
      atMedium,
      facet('priority', 'L', 'priority L'),
      atNone,
      inOps,
      inWorkflow,
      inNoProject,
      taggedJira,
      untagged,
      linked,
      unlinked,
    ],
  }
}

// mockTasksSummary is what the mockup's stream carries of those tasks: the
// started one, and the two linked to issues.
export const mockTasksSummary: TasksSummary = {
  available: true,
  reason: '',
  active: redacting,
  linked: [redacting, caching],
}

// linkedDone is each task linked to an issue as the server describes it once
// marked done, beside what it held before: its place among the linked tasks,
// and the fields typed text matches, no longer its #id, since a done task
// leaves Taskwarrior's working set.
const linkedDone = new Map<string, Pick<Task, 'ranks' | 'searchable'>>([
  [
    redacting.uuid,
    {
      ranks: { urgency: 0, state: 1, id: 1, tag: 0, issue: 1, priority: 0 },
      searchable: ['proj-412: fix token redaction', 'workflow', 'proj-412', '+jira'],
    },
  ],
  [
    caching.uuid,
    {
      ranks: { urgency: 1, state: 1, id: 1, tag: 1, issue: 0, priority: 1 },
      searchable: [
        'proj-408: cache the forge ci status between polls',
        'workflow',
        'proj-408',
        '+jira',
      ],
    },
  ],
])

// mockDone is what the mockup answers a done of the task uuid names with, as
// the server does: the list without it, the places behind it closed up, and,
// for a task linked to an issue, the task as it stands done — stopped, out of
// the working set and completed.
function mockDone(uuid: string): TaskList {
  const list = mockTaskList()
  const gone = list.tasks.find((task) => task.uuid === uuid)
  if (gone === undefined) {
    return list
  }

  const described = linkedDone.get(uuid)
  const tasks = list.tasks.filter((task) => task !== gone).map((task) => closedUp(task, gone.ranks))
  if (described === undefined) {
    return { ...list, tasks }
  }

  const done: Task = {
    ...gone,
    ...described,
    id: 0,
    status: 'completed',
    state: 'completed',
    start: undefined,
    end: at(0),
    modified: at(0),
    facets: gone.facets.map((held) => (held.kind === 'state' ? isCompleted : held)),
  }

  return { ...list, tasks, done }
}

// closedUp is task in a list a task at gone has left: its place in each order
// behind that one moves up one, as the server's places among the rest are.
function closedUp(task: Task, gone: TaskRanks): Task {
  const ranks = { ...task.ranks }
  for (const order of ['urgency', 'state', 'id', 'tag', 'issue', 'priority'] as const) {
    if (ranks[order] > gone[order]) {
      ranks[order] -= 1
    }
  }

  return { ...task, ranks }
}

// A TaskRoute answers one of the task routes the mockup's server takes, from
// the named parts of its path.
type TaskRoute = (asked: { params: Record<string, string> }) => TaskList

// taskRoutes are the mockup server's routes for your tasks, by method and path:
// the list, and every write on it, which answers the list as it stands; a done
// answers it without the task, and the task as it stands done.
export function taskRoutes(): Record<string, TaskRoute> {
  const listed = () => mockTaskList()

  return {
    ...Object.fromEntries(
      [
        'GET /api/tasks',
        'POST /api/tasks',
        'POST /api/tasks/track',
        'POST /api/tasks/undo',
        'POST /api/tasks/sync',
        ...['start', 'stop', 'annotations', 'modify'].map(
          (write) => `POST /api/tasks/{uuid}/${write}`,
        ),
      ].map((name) => [name, listed]),
    ),
    'POST /api/tasks/{uuid}/done': ({ params }) => mockDone(params.uuid ?? ''),
  }
}
