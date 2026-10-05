import type { Activity, ActivityDay, ActivityItem } from '../api/generated/types.gen.ts'
import type { Period } from '../features/summary/civilDate.ts'

// mockActivity is a working day for the mockup, whatever period is asked: a
// commit, a task finished, an issue moved and a pull request opened, with
// Jira's comments unread, so every hue and the source note are on screen.
export function mockActivity(period: Period | null): Activity {
  const from = period?.from ?? '2026-09-15'

  return {
    from,
    to: period?.to ?? from,
    today: '2026-09-16',
    sources: [
      { source: 'git', name: 'Git', failed: false, truncated: false, detail: '' },
      { source: 'tasks', name: 'Taskwarrior', failed: false, truncated: false, detail: '' },
      { source: 'jira', name: 'Jira', failed: false, truncated: true, detail: '' },
      { source: 'forge', name: 'The forge', failed: false, truncated: false, detail: '' },
    ],
    years: [
      {
        year: Number(from.slice(0, 4)),
        months: [{ month: Number(from.slice(5, 7)), name: 'September', days: [mockDay(from)] }],
      },
    ],
    text: `# ${from}\n\n- committed 4f2c9e1 fix(config): redact tokens before they reach the log\n`,
  }
}

// mockItem is one thing done on date at a time of day; only the forge's is in
// a repository.
function mockItem(
  date: string,
  at: string,
  fields: Pick<ActivityItem, 'source' | 'verb' | 'ref' | 'title' | 'url'>,
): ActivityItem {
  return {
    at: `${date}T${at}:00Z`,
    ...fields,
    repository: fields.source === 'forge' ? 'acme/workflow' : '',
  }
}

// mockDay is the day's two hours: a commit and a task done at nine, an issue
// moved and a pull request opened at two.
function mockDay(date: string): ActivityDay {
  const nine = [
    mockItem(date, '09:12', {
      source: 'git',
      verb: 'committed',
      ref: '4f2c9e1',
      title: 'fix(config): redact tokens before they reach the log',
      url: '',
    }),
    mockItem(date, '09:40', {
      source: 'tasks',
      verb: 'completed task',
      ref: '12',
      title: 'Write the redaction test',
      url: '',
    }),
  ]
  const two = [
    mockItem(date, '14:05', {
      source: 'jira',
      verb: 'moved',
      ref: 'PROJ-412',
      title: 'Redact tokens before they reach the request log, to In Review',
      url: 'https://jira.example.com/browse/PROJ-412',
    }),
    mockItem(date, '14:20', {
      source: 'forge',
      verb: 'opened',
      ref: 'acme/workflow#42',
      title: 'fix(config): redact tokens',
      url: 'https://github.com/acme/workflow/pull/42',
    }),
  ]

  return {
    date,
    weekday: 'Tuesday',
    hours: [
      { label: '09:00', items: nine },
      { label: '14:00', items: two },
    ],
  }
}
