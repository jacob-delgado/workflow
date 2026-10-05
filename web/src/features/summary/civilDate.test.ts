import {
  addDays,
  addMonths,
  monthOf,
  monthWeeks,
  parseDate,
  periodDays,
  shiftPeriod,
  yearOf,
} from './civilDate.ts'

// The twin of internal/activity's dates: each case here has a case of the same
// name in period_test.go, so the two cannot drift apart unnoticed (TRADE-33).

test.each([
  ['a date', '2026-02-28', true],
  ['no such day', '2026-02-30', false],
  ['not a date', 'yesterday', false],
  ['with a time', '2026-02-28T10:00:00Z', false],
  ['leap day kept', '2028-02-29', true],
])('a date is read and written as year month day: %s', (_, text, ok) => {
  // Act
  const date = parseDate(text)

  // Assert
  expect(date).toBe(ok ? text : null)
})

test.each([
  ['a day every month has', '2026-01-15', 1, '2026-02-15'],
  ['the 31st into a short one', '2026-01-31', 1, '2026-02-28'],
  ['into a leap February', '2028-01-31', 1, '2028-02-29'],
  ["back over a year's end", '2026-01-31', -2, '2025-11-30'],
  ['a year on from a leap day', '2028-02-29', 12, '2029-02-28'],
])('moving by months keeps the day where the month has it: %s', (_, from, months, want) => {
  // Act & Assert
  expect(addMonths(from, months)).toBe(want)
})

test("a date's month and year are whole periods", () => {
  // Act
  const month = monthOf('2028-02-10')
  const year = yearOf('2028-02-10')

  // Assert
  expect(month).toEqual({ from: '2028-02-01', to: '2028-02-29' })
  expect(year).toEqual({ from: '2028-01-01', to: '2028-12-31' })
})

test('a day moves over the end of a month and a year', () => {
  // Act & Assert
  expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
})

test('a period shifts back or on by its own length', () => {
  // Arrange
  const weekend = { from: '2026-09-11', to: '2026-09-13' }

  // Act
  const earlier = shiftPeriod(weekend, -1)

  // Assert
  expect(periodDays(weekend)).toBe(3)
  expect(earlier).toEqual({ from: '2026-09-08', to: '2026-09-10' })
})

test.each([
  [
    'a whole month, back',
    { from: '2026-10-01', to: '2026-10-31' },
    -1,
    { from: '2026-09-01', to: '2026-09-30' },
  ],
  [
    'a whole month, on',
    { from: '2026-01-01', to: '2026-01-31' },
    1,
    { from: '2026-02-01', to: '2026-02-28' },
  ],
  [
    'a whole year, back',
    { from: '2026-01-01', to: '2026-12-31' },
    -1,
    { from: '2025-01-01', to: '2025-12-31' },
  ],
] as const)('%s shifts by itself, whatever its days', (_, period, direction, want) => {
  // Act
  const shifted = shiftPeriod(period, direction)

  // Assert
  expect(shifted).toEqual(want)
})

test('a month is weeks starting on Monday, blank before its first day', () => {
  // Act
  const weeks = monthWeeks(2026, 9)

  // Assert
  // September 2026 starts on a Tuesday and ends on a Wednesday.
  expect(weeks[0]).toEqual([
    null,
    '2026-09-01',
    '2026-09-02',
    '2026-09-03',
    '2026-09-04',
    '2026-09-05',
    '2026-09-06',
  ])
  expect(weeks.at(-1)).toEqual(['2026-09-28', '2026-09-29', '2026-09-30', null, null, null, null])
})
