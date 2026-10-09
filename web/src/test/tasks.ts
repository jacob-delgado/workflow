import type { Task, TaskRanks } from '@/api/generated/types.gen.ts'
import { describedTask, standing, taskFacet, taskStanding } from './fixtures.ts'

// The tasks the Tasks section's and the issue's Tasks card's cases share, each
// as the server answers it: what Taskwarrior holds of it and what the server
// describes, written out together. Each carries the ranks of the only task of
// its list, first in every order; a case that lists several together gives
// each its place in every order of that list with ranked, written out as the
// server ranks it.

// ranked is task at the places the server gives it in each order of the list
// a case answers it in.
export function ranked(task: Task, ranks: TaskRanks): Task {
  return { ...task, ranks }
}

// firstInEveryOrder is the place of a task first in every order of its list.
export const firstInEveryOrder: TaskRanks = {
  urgency: 0,
  state: 0,
  id: 0,
  tag: 0,
  issue: 0,
  priority: 0,
}

// secondInEveryOrder is the place of a task second in every order of its
// list.
export const secondInEveryOrder: TaskRanks = {
  urgency: 1,
  state: 1,
  id: 1,
  tag: 1,
  issue: 1,
  priority: 1,
}

const hour = 3_600_000

// unlinkedFacets are the values a pending task with no priority, project, tag
// or issue holds, after its state.
const unlinkedFacets = [
  taskFacet.noPriority,
  taskFacet.noProject,
  taskFacet.noIssue,
  taskFacet.noTag,
] as const

// linkedFacets are the values a pending task with no priority, project or tag,
// tracking an issue, holds, after its state.
const linkedFacets = [
  taskFacet.noPriority,
  taskFacet.noProject,
  taskFacet.withIssue,
  taskFacet.noTag,
] as const

// tokenLeak is task 1, pending, tracking PROJ-1, the most urgent.
export const tokenLeak: Task = describedTask(
  {
    uuid: '11111111-1111-4111-8111-111111111111',
    id: 1,
    description: 'PROJ-1: Fix the token leak',
    status: 'pending',
    project: '',
    priority: '',
    tags: [],
    issue_key: 'PROJ-1',
    issue_url: 'https://jira.example.com/browse/PROJ-1',
    urgency: 9.5,
  },
  {
    state: 'pending',
    facets: [taskFacet.pending, ...linkedFacets],
    searchable: ['proj-1: fix the token leak', '', 'proj-1', '#1'],
  },
)

// startedTokenLeak is the token leak, started at start.
export function startedTokenLeak(start: string): Task {
  return taskStanding(tokenLeak, { start }, standing.started)
}

// certificate is task 2, pending, tracking no issue.
export const certificate: Task = describedTask(
  {
    uuid: '22222222-2222-4222-8222-222222222222',
    id: 2,
    description: 'Renew the certificate',
    status: 'pending',
    project: '',
    priority: '',
    tags: [],
    issue_key: '',
    issue_url: '',
    urgency: 5.1,
  },
  {
    state: 'pending',
    facets: [taskFacet.pending, ...unlinkedFacets],
    searchable: ['renew the certificate', '', '', '#2'],
  },
)

// cacheTuning is task 3, pending, tracking PROJ-9.
export const cacheTuning: Task = describedTask(
  {
    uuid: '33333333-3333-4333-8333-333333333333',
    id: 3,
    description: 'PROJ-9: Tune the cache',
    status: 'pending',
    project: '',
    priority: '',
    tags: [],
    issue_key: 'PROJ-9',
    issue_url: 'https://jira.example.com/browse/PROJ-9',
    urgency: 3.2,
  },
  {
    state: 'pending',
    facets: [taskFacet.pending, ...linkedFacets],
    searchable: ['proj-9: tune the cache', '', 'proj-9', '#3'],
  },
)

// retroRoom is task 4, waiting two days, tracking no issue.
export const retroRoom: Task = describedTask(
  {
    uuid: '44444444-4444-4444-8444-444444444444',
    id: 4,
    description: 'Book the retro room',
    status: 'waiting',
    wait: new Date(Date.now() + 48 * hour).toISOString(),
    project: '',
    priority: '',
    tags: [],
    issue_key: '',
    issue_url: '',
    urgency: 1.1,
  },
  {
    state: 'waiting',
    facets: [taskFacet.waiting, ...unlinkedFacets],
    searchable: ['book the retro room', '', '', '#4'],
  },
)

// tracking is task 12, pending, tracking PROJ-412.
export const tracking: Task = describedTask(
  {
    id: 12,
    description: 'PROJ-412: Redact tokens before they reach the request log',
    status: 'pending',
    project: '',
    priority: '',
    tags: [],
    issue_key: 'PROJ-412',
    issue_url: 'https://jira.example.com/browse/PROJ-412',
  },
  {
    state: 'pending',
    facets: [taskFacet.pending, ...linkedFacets],
    searchable: [
      'proj-412: redact tokens before they reach the request log',
      '',
      'proj-412',
      '#12',
    ],
  },
)

// trackingDone is tracking, marked done at end: out of Taskwarrior's working
// set, so numbered 0, which typed text no longer matches.
export function trackingDone(end: string): Task {
  return describedTask(
    { ...tracking, id: 0, status: 'completed', end },
    {
      state: 'completed',
      facets: [taskFacet.completed, ...linkedFacets],
      searchable: ['proj-412: redact tokens before they reach the request log', '', 'proj-412'],
    },
  )
}
