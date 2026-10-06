// Every date the web writes is written here, in one locale: the interface's
// copy is English, so its dates read the way that copy does, whatever the
// browser reports. Two styles, one for running text and one for a heading,
// so the same day reads the same a section apart.
const locale = 'en-US'

const runningText: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric', year: 'numeric' }

const heading: Intl.DateTimeFormatOptions = {
  weekday: 'long',
  month: 'long',
  day: 'numeric',
  year: 'numeric',
}

// writtenDate is a moment's date, in the browser's time zone, as running text
// says it: Sep 18, 2026.
export function writtenDate(at: Date): string {
  return at.toLocaleDateString(locale, runningText)
}

// writtenDay is a calendar date, YYYY-MM-DD, as running text says it.
export function writtenDay(date: string): string {
  return civilNoon(date).toLocaleDateString(locale, { ...runningText, timeZone: 'UTC' })
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

// civilNoon is noon UTC on a calendar date, so formatting it in UTC can never
// land on the day before or after.
function civilNoon(date: string): Date {
  const [year, month, day] = date.split('-').map(Number)

  return new Date(Date.UTC(year ?? 1, (month ?? 1) - 1, day ?? 1, 12))
}
