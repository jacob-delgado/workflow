// Every date the web writes is written here, in one locale: the interface's
// copy is English, so its dates read the way that copy does, whatever the
// browser reports. Two styles, one for running text — YYYY-MM-DD, as the
// terminal writes every date — and one for a heading, so the same day reads
// the same a section apart, and the same on both interfaces.
const locale = 'en-US'

const heading: Intl.DateTimeFormatOptions = {
  weekday: 'long',
  month: 'long',
  day: 'numeric',
  year: 'numeric',
}

// writtenDate is a moment's date, in the browser's time zone, as running text
// says it: 2026-09-18, the terminal's time.DateOnly.
export function writtenDate(at: Date): string {
  const month = String(at.getMonth() + 1).padStart(2, '0')
  const day = String(at.getDate()).padStart(2, '0')

  return `${String(at.getFullYear())}-${month}-${day}`
}

// writtenDay is a calendar date, YYYY-MM-DD, as running text says it.
export function writtenDay(date: string): string {
  return civilNoon(date).toISOString().slice(0, 10)
}

// headingDay is a calendar date, YYYY-MM-DD, as a heading or a day's
// accessible name says it: Tuesday, September 15, 2026.
export function headingDay(date: string): string {
  return civilNoon(date).toLocaleDateString(locale, { ...heading, timeZone: 'UTC' })
}

// monthHeading is a month (1–12) of a year as a heading names it: September
// 2026.
export function monthHeading(year: number, month: number): string {
  return new Date(Date.UTC(year, month - 1, 1, 12)).toLocaleDateString(locale, {
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  })
}

// writtenMoment is a moment's whole date and its time, for where the exact
// moment is asked for, such as on hover.
export function writtenMoment(at: Date): string {
  return at.toLocaleString(locale, { dateStyle: 'full', timeStyle: 'short' })
}

// relativeTime is a count of units before (negative) or after now, in words:
// 2 days ago, yesterday.
export function relativeTime(count: number, unit: Intl.RelativeTimeFormatUnit): string {
  return new Intl.RelativeTimeFormat(locale, { numeric: 'auto' }).format(count, unit)
}

// The lengths of time the web counts in, in milliseconds.
export const second = 1_000
export const minute = 60 * second
export const hour = 60 * minute
export const day = 24 * hour

// AgoStyle is how a time ago is worded: narrow, a figure for a line with no
// room for words (3s ago, 2h ago); terminal, as the terminal's lists say it
// (5m ago, 3d ago); words, in running text (5 minutes ago, yesterday).
type AgoStyle = 'narrow' | 'terminal' | 'words'

interface AgoOptions {
  style: AgoStyle
  // dateAfter is how long ago a moment may be before its date is said
  // instead, YYYY-MM-DD; never, unless given.
  dateAfter?: number
}

// ago is how long before now a moment was, both in milliseconds since the
// epoch, in the style asked for. Under a minute it is just now, but for the
// narrow style, which counts seconds; and a moment at or before the epoch —
// the zero time a server sends for a date it was not given — is some time ago.
export function ago(at: number, now: number, { style, dateAfter = Infinity }: AgoOptions): string {
  const elapsed = now - at
  if (at <= 0) {
    return 'some time ago'
  }

  if (elapsed >= dateAfter) {
    return writtenDate(new Date(at))
  }

  switch (style) {
    case 'narrow':
      return narrowAgo(elapsed)
    case 'terminal':
      return terminalAgo(elapsed)
    case 'words':
      return wordsAgo(elapsed)
  }
}

// narrowAgo is a time ago as a figure: just now, then 3s ago, 5m ago, 2h ago.
function narrowAgo(elapsed: number): string {
  if (elapsed < second) {
    return 'just now'
  }

  const narrow = new Intl.RelativeTimeFormat(locale, { numeric: 'always', style: 'narrow' })
  if (elapsed < minute) {
    return narrow.format(-Math.floor(elapsed / second), 'second')
  }

  if (elapsed < hour) {
    return narrow.format(-Math.floor(elapsed / minute), 'minute')
  }

  return narrow.format(-Math.floor(elapsed / hour), 'hour')
}

// terminalAgo is a time ago as the terminal's lists say it: just now, then
// 5m ago, 2h ago, 3d ago.
function terminalAgo(elapsed: number): string {
  if (elapsed < minute) {
    return 'just now'
  }

  if (elapsed < hour) {
    return `${String(Math.floor(elapsed / minute))}m ago`
  }

  return elapsed < day
    ? `${String(Math.floor(elapsed / hour))}h ago`
    : `${String(Math.floor(elapsed / day))}d ago`
}

// wordsAgo is a time ago in words: just now, then 5 minutes ago, 2 hours ago,
// yesterday, 3 days ago.
function wordsAgo(elapsed: number): string {
  if (elapsed < minute) {
    return 'just now'
  }

  if (elapsed < hour) {
    return relativeTime(-Math.floor(elapsed / minute), 'minute')
  }

  return elapsed < day
    ? relativeTime(-Math.floor(elapsed / hour), 'hour')
    : relativeTime(-Math.floor(elapsed / day), 'day')
}

// civilNoon is noon UTC on a calendar date, so formatting it in UTC can never
// land on the day before or after.
function civilNoon(date: string): Date {
  const [year, month, day] = date.split('-').map(Number)

  return new Date(Date.UTC(year ?? 1, (month ?? 1) - 1, day ?? 1, 12))
}
