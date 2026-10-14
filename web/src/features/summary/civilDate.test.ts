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
  yearMonth,
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
    weeks: z.array(
      z.strictObject({
        name: z.string(),
        date: z.string(),
        weeks: z.array(z.array(z.string().nullable())),
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

test.each(corpus.weeks)(
  'a month is weeks starting on Monday, blank outside it: $name',
  ({ date, weeks }) => {
    // Act & Assert
    expect(monthWeeks(...yearMonth(date))).toEqual(weeks)
  },
)
