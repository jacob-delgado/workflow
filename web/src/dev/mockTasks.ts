import type { Task, TaskList, TasksSummary } from '@/api/generated/types.gen.ts'
import { minute } from '@/lib/dates.ts'

// at is the RFC 3339 time some minutes from now — before it when negative — so
// the mockup's ages and due dates read the same whenever it is shown.
function at(minutes: number): string {
  return new Date(Date.now() + minutes * minute).toISOString()
}

const hours = 60
const days = 24 * hours

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
}

// mockTaskList is the task list `task web:mockup` shows: a started task for the
// checked-out issue, one for another issue, one no issue tracks and one
// waiting, most urgent first, with a sync backend to offer Sync. Dev-only, and
// code-split out of a production build.
export function mockTaskList(): TaskList {
  return {
    available: true,
    reason: '',
    context: '',
    sync_available: true,
    said: '',
    tasks: [redacting, caching, certificate, retro],
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
