import type {
  Activity,
  ActivityDay,
  ActivityItem,
  ActivityYear,
} from '../api/generated/types.gen.ts'

// today is the day the mockup's server says it is, and lastWorkingDay the one
// it reads back when no period is asked.
const today = '2026-09-16'
const lastWorkingDay = '2026-09-15'

// workload is how many of a working day's four things are done on a weekday,
// by its day of the month: busy days and quiet ones, so a month's calendar is
// tinted at every level. The last working day, the 15th, does all four.
const workload = [4, 1, 3, 0, 2] as const

// dayLength is a day in milliseconds; dates are counted at noon in UTC, which
// keeps no clock change.
const dayLength = 24 * 60 * 60 * 1000

// mockActivity is what the mockup did over the period asked, or on the last
// working day when none is: on each weekday up to today, some or all of a
// working day — a commit, a task finished, an issue moved and a pull request
// opened — with Jira's comments unread, so every hue and the source note are
// on screen. It reads the period as the answer names one, so the e2e specs
// that import it type-check it without the app's own modules.
export function mockActivity(period: Pick<Activity, 'from' | 'to'> | null): Activity {
  const from = period?.from ?? lastWorkingDay
  const to = period?.to ?? from
  const days = datesFrom(from, to)
    .map(mockDay)
    .filter((day) => day.hours.length > 0)

  return {
    from,
    to,
    today,
    sources: [
      { source: 'git', name: 'Git', state: 'read', truncated: false, detail: '' },
      { source: 'tasks', name: 'Taskwarrior', state: 'read', truncated: false, detail: '' },
      { source: 'jira', name: 'Jira', state: 'read', truncated: true, detail: '' },
      { source: 'forge', name: 'The forge', state: 'read', truncated: false, detail: '' },
    ],
    years: byMonth(days),
    text:
      days.length === 0
        ? `# ${from}\n\nNothing was done in this period.\n`
        : `# ${from}\n\n- committed 4f2c9e1 fix(config): redact tokens before they reach the log\n`,
  }
}

// noonOf is a date written YYYY-MM-DD, at noon in UTC.
function noonOf(date: string): Date {
  return new Date(`${date}T12:00:00Z`)
}

// datesFrom are the dates from from through to, oldest first.
function datesFrom(from: string, to: string): string[] {
  const dates: string[] = []
  for (let at = noonOf(from).getTime(); at <= noonOf(to).getTime(); at += dayLength) {
    dates.push(new Date(at).toISOString().slice(0, 10))
  }

  return dates
}

// amountOn is how many things the mockup did on date: none on a weekend or
// after today, and its workload on any other day.
function amountOn(date: string): number {
  const weekday = noonOf(date).getUTCDay()
  if (weekday === 0 || weekday === 6 || date > today) {
    return 0
  }

  return workload[Number(date.slice(8)) % workload.length] ?? 0
}

// byMonth groups days, oldest first, by year and month, as the server does.
function byMonth(days: ActivityDay[]): ActivityYear[] {
  const years: ActivityYear[] = []
  for (const day of days) {
    const [year, month] = [Number(day.date.slice(0, 4)), Number(day.date.slice(5, 7))]
    if (years.at(-1)?.year !== year) {
      years.push({ year, months: [] })
    }

    const months = years.at(-1)?.months ?? []
    if (months.at(-1)?.month !== month) {
      months.push({ month, name: nameOf(day.date, 'month'), days: [] })
    }

    months.at(-1)?.days.push(day)
  }

  return years
}

// nameOf is the name of a date's month or weekday, as the server writes it.
function nameOf(date: string, part: 'month' | 'weekday'): string {
  return noonOf(date).toLocaleDateString('en-US', { [part]: 'long', timeZone: 'UTC' })
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

// mockDay is what was done on date, in the order a working day does it: a
// commit and a task done at nine, an issue moved and a pull request opened at
// two, as many of them as the day's amount, each hour with nothing left out.
function mockDay(date: string): ActivityDay {
  const done = workingDay(date).slice(0, amountOn(date))
  const hours = ['09', '14'].map((hour) => ({
    label: `${hour}:00`,
    items: done.filter((item) => item.at.slice(11, 13) === hour),
  }))

  return {
    date,
    weekday: nameOf(date, 'weekday'),
    hours: hours.filter((hour) => hour.items.length > 0),
  }
}

// workingDay is everything the mockup does on a full working day, in order.
function workingDay(date: string): ActivityItem[] {
  return [
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
}
