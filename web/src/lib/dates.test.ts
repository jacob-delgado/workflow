import {
  ago,
  civilDay,
  civilNoon,
  day,
  headingDay,
  hour,
  minute,
  monthHeading,
  relativeTime,
  second,
  writtenCount,
  writtenDate,
  writtenDay,
  writtenMoment,
} from './dates.ts'

// noonOn is noon local time on a day, so its date is that day in any zone.
function noonOn(year: number, month: number, day: number): Date {
  return new Date(year, month - 1, day, 12)
}

test('a date in running text is written as the terminal writes it', () => {
  // Act & Assert
  expect(writtenDate(noonOn(2026, 9, 18))).toBe('2026-09-18')
})

test('a civil date in running text reads the same as that day as a moment', () => {
  // Act & Assert
  expect(writtenDay('2026-09-18')).toBe(writtenDate(noonOn(2026, 9, 18)))
})

test('a day heading names the weekday and the whole date', () => {
  // Act & Assert
  expect(headingDay('2026-09-15')).toBe('Tuesday, September 15, 2026')
})

test('a month heading names the month and the year', () => {
  // Act & Assert
  expect(monthHeading(2026, 9)).toBe('September 2026')
})

test('a moment names its date and its time', () => {
  // Act
  const moment = writtenMoment(new Date(2026, 8, 18, 15, 4))

  // Assert
  expect(moment).toMatch(/^Friday, September 18, 2026 at 3:04\sPM$/)
})

test('a time relative to now is said in words', () => {
  // Act & Assert
  expect(relativeTime(-2, 'day')).toBe('2 days ago')
})

// now is the moment every time ago below is told from.
const now = noonOn(2026, 9, 18).getTime()

test.each([
  [400, 'just now'],
  [3 * second, '3s ago'],
  [59_999, '59s ago'],
  [5 * minute, '5m ago'],
  [2 * hour + 1, '2h ago'],
  [3 * day, '72h ago'],
])('%i ms ago, in a figure, is %s', (elapsed, words) => {
  // Act & Assert
  expect(ago(now - elapsed, now, { style: 'narrow' })).toBe(words)
})

test.each([
  [30 * second, 'just now'],
  [15 * minute, '15m ago'],
  [5 * hour, '5h ago'],
  [9 * day, '9d ago'],
  [31 * day, '2026-08-18'],
])('%i ms ago, as the terminal says it, is %s', (elapsed, words) => {
  // Act & Assert
  expect(ago(now - elapsed, now, { style: 'terminal', dateAfter: 30 * day })).toBe(words)
})

test.each([
  [30 * second, 'just now'],
  [5 * minute, '5 minutes ago'],
  [2 * hour, '2 hours ago'],
  [day + hour, 'yesterday'],
  [3 * day, '3 days ago'],
  [8 * day, '2026-09-10'],
])('%i ms ago, in words, is %s', (elapsed, words) => {
  // Act & Assert
  expect(ago(now - elapsed, now, { style: 'words', dateAfter: 7 * day })).toBe(words)
})

test('a moment at the epoch is counted like any other, no stand-in for an unknown time', () => {
  // Act & Assert
  expect(ago(0, 3 * hour, { style: 'narrow' })).toBe('3h ago')
})

test.each(['not a date', '2026-02-30', '2026-9-5', ''])(
  'a calendar date the server got wrong, "%s", is written as it came',
  (date) => {
    // Act & Assert
    expect(writtenDay(date)).toBe(date)
  },
)

test.each(['not a date', '2026-13-01'])(
  'a heading for "%s", no calendar date, is the text',
  (date) => {
    // Act & Assert
    expect(headingDay(date)).toBe(date)
  },
)

test('noon on a calendar day is that day in UTC', () => {
  // Act & Assert
  expect(civilDay(civilNoon(2026, 2, 28))).toBe('2026-02-28')
})

test('a count is written with its thousands marked, as the copy writes numbers', () => {
  // Act & Assert
  expect(writtenCount(1_234_567)).toBe('1,234,567')
})
