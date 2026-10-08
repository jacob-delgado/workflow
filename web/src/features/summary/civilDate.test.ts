import { z } from 'zod'
import { twinCases } from '@/test/twinCases.ts'
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

// The cases internal/activity's dates answer to as well.
const period = z.strictObject({ from: z.string(), to: z.string() })
const corpus = twinCases(
  'internal/activity/testdata/civil_dates.json',
  z.strictObject({
    about: z.string(),
    parse: z.array(z.strictObject({ name: z.string(), text: z.string(), ok: z.boolean() })),
    add_days: z.array(
      z.strictObject({ name: z.string(), from: z.string(), days: z.number(), want: z.string() }),
    ),
    add_months: z.array(
      z.strictObject({ name: z.string(), from: z.string(), months: z.number(), want: z.string() }),
    ),
    whole: z.array(
      z.strictObject({ name: z.string(), date: z.string(), month: period, year: period }),
    ),
    step: z.array(
      z.strictObject({
        name: z.string(),
        from: z.string(),
        to: z.string(),
        steps: z.union([z.literal(-1), z.literal(1)]),
        days: z.number(),
        want: period,
      }),
    ),
  }),
)

test.each(corpus.parse)('a date is read and written as year month day: $name', ({ text, ok }) => {
  // Act
  const date = parseDate(text)

  // Assert
  expect(date).toBe(ok ? text : null)
})

test.each(corpus.add_days)('moving by days: $name', ({ from, days, want }) => {
  // Act & Assert
  expect(addDays(from, days)).toBe(want)
})

test.each(corpus.add_months)(
  'moving by months keeps the day where the month has it: $name',
  ({ from, months, want }) => {
    // Act & Assert
    expect(addMonths(from, months)).toBe(want)
  },
)

test.each(corpus.whole)("a date's month and year are whole periods: $name", (whole) => {
  // Act
  const month = monthOf(whole.date)
  const year = yearOf(whole.date)

  // Assert
  expect({ month, year }).toEqual({ month: whole.month, year: whole.year })
})

test.each(corpus.step)('a period steps by its own length: $name', (step) => {
  // Arrange
  const stepping = { from: step.from, to: step.to }

  // Act
  const stepped = shiftPeriod(stepping, step.steps)

  // Assert
  expect({ stepped, days: periodDays(stepping) }).toEqual({ stepped: step.want, days: step.days })
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
