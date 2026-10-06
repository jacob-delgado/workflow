// Trade-off TRADE-33: these are internal/activity's dates written again, so
// the calendar can move and pick a period with no request; twin-named cases in
// civilDate.test.ts and period_test.go pin the two together.

// A CivilDate is a day on the calendar, written YYYY-MM-DD, with no time and
// no zone: the days a period names are the same whichever clock reads them.
export type CivilDate = string

// A Period is the days from from through to, both included.
export interface Period {
  from: CivilDate
  to: CivilDate
}

const dayMs = 24 * 60 * 60 * 1000
const datePattern = /^(\d{4})-(\d{2})-(\d{2})$/

// noon is the date at noon UTC, an instant no clock change moves off its day.
function noon(year: number, month: number, day: number): Date {
  return new Date(Date.UTC(year, month - 1, day, 12))
}

// written is an instant's day in UTC, YYYY-MM-DD.
function written(instant: Date): CivilDate {
  return instant.toISOString().slice(0, 10)
}

// parts are a date's year, month (1–12) and day.
function parts(date: CivilDate): [number, number, number] {
  const [year = 0, month = 0, day = 0] = date.split('-').map(Number)

  return [year, month, day]
}

// parseDate reads a date written YYYY-MM-DD, or null for text that is not
// one or names a day its month does not have.
export function parseDate(text: string): CivilDate | null {
  const match = datePattern.exec(text)
  if (match === null) {
    return null
  }

  const [year, month, day] = parts(text)

  return written(noon(year, month, day)) === text ? text : null
}

// addDays is the date days later, or earlier when days is negative.
export function addDays(date: CivilDate, days: number): CivilDate {
  const [year, month, day] = parts(date)

  return written(new Date(noon(year, month, day).getTime() + days * dayMs))
}

// daysIn is how many days a month (1–12) of a year has.
function daysIn(year: number, month: number): number {
  return new Date(Date.UTC(year, month, 0, 12)).getUTCDate()
}

// addMonths is the date months later, or earlier when months is negative, on
// the same day of the month, or the month's last where it has no such day.
export function addMonths(date: CivilDate, months: number): CivilDate {
  const [year, month, day] = parts(date)
  const first = noon(year, month + months, 1)
  const toYear = first.getUTCFullYear()
  const toMonth = first.getUTCMonth() + 1

  return written(noon(toYear, toMonth, Math.min(day, daysIn(toYear, toMonth))))
}

// monthOf is the whole month a date falls in.
export function monthOf(date: CivilDate): Period {
  const [year, month] = parts(date)

  return {
    from: written(noon(year, month, 1)),
    to: written(noon(year, month, daysIn(year, month))),
  }
}

// yearOf is the whole year a date falls in.
export function yearOf(date: CivilDate): Period {
  const [year] = parts(date)

  return { from: written(noon(year, 1, 1)), to: written(noon(year, 12, 31)) }
}

// periodDays is how many days a period holds.
export function periodDays(period: Period): number {
  const [fromYear, fromMonth, fromDay] = parts(period.from)
  const [toYear, toMonth, toDay] = parts(period.to)

  return (
    Math.round(
      (noon(toYear, toMonth, toDay).getTime() - noon(fromYear, fromMonth, fromDay).getTime()) /
        dayMs,
    ) + 1
  )
}

// shiftPeriod is the period moved back (direction -1) or on (1) by its own
// length. A whole year moves a year, and a whole month a month, so a month
// stays a month whatever its days.
export function shiftPeriod(period: Period, direction: -1 | 1): Period {
  if (samePeriod(period, yearOf(period.from))) {
    return yearOf(addMonths(period.from, direction * monthsInYear))
  }

  if (samePeriod(period, monthOf(period.from))) {
    return monthOf(addMonths(period.from, direction))
  }

  const days = periodDays(period) * direction

  return { from: addDays(period.from, days), to: addDays(period.to, days) }
}

// monthsInYear is how many months a whole year moves by.
const monthsInYear = 12

// samePeriod reports two periods run over the same days.
function samePeriod(one: Period, other: Period): boolean {
  return one.from === other.from && one.to === other.to
}

// monthWeeks are a month's days as weeks starting on Monday, a null for each
// place before its first day and after its last.
export function monthWeeks(year: number, month: number): (CivilDate | null)[][] {
  const first = noon(year, month, 1)
  const lead = (first.getUTCDay() + 6) % 7
  const days: (CivilDate | null)[] = Array.from({ length: lead }, () => null)

  for (let day = 1; day <= daysIn(year, month); day++) {
    days.push(written(noon(year, month, day)))
  }

  while (days.length % 7 !== 0) {
    days.push(null)
  }

  const weeks: (CivilDate | null)[][] = []
  for (let start = 0; start < days.length; start += 7) {
    weeks.push(days.slice(start, start + 7))
  }

  return weeks
}

// yearMonth is a date's year and month (1–12).
export function yearMonth(date: CivilDate): [number, number] {
  const [year, month] = parts(date)

  return [year, month]
}
