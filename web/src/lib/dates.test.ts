import {
  headingDay,
  monthHeading,
  relativeTime,
  writtenDate,
  writtenDay,
  writtenMoment,
} from './dates.ts'

// noonOn is noon local time on a day, so its date is that day in any zone.
function noonOn(year: number, month: number, day: number): Date {
  return new Date(year, month - 1, day, 12)
}

test('a date in running text is short and unambiguous', () => {
  // Act & Assert
  expect(writtenDate(noonOn(2026, 9, 18))).toBe('Sep 18, 2026')
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
