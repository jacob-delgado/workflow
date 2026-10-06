import { useId, useState } from 'react'
import { Button } from '@/lib/Button.tsx'
import { Select } from '@/lib/Field.tsx'
import {
  monthOf,
  shiftPeriod,
  yearMonth,
  yearOf,
  type CivilDate,
  type Period,
} from './civilDate.ts'
import { MonthGrid } from './MonthGrid.tsx'

const monthNames = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
] as const

// yearsBefore is how many years the Year select offers before the one shown.
const yearsBefore = 5

interface PeriodPickerProps {
  period: Period
  today: CivilDate
  onPick: (period: Period) => void
}

// PeriodCalendar picks the period the Summary shows from a month's calendar —
// a day, a range, the month or its year — the month and year chosen above it.
export function PeriodCalendar({ period, today, onPick }: PeriodPickerProps) {
  const [view, setView] = useState<[number, number]>(() => yearMonth(period.to))
  const [year, month] = view
  const shownDay = `${String(year)}-${String(month).padStart(2, '0')}-01`

  return (
    <section aria-label="Calendar" className="flex flex-col gap-group">
      <MonthSelects
        year={year}
        month={month}
        thisYear={yearMonth(today)[0]}
        onView={(nextYear, nextMonth) => {
          setView([nextYear, nextMonth])
        }}
      />
      <MonthGrid
        year={year}
        month={month}
        period={period}
        onPick={onPick}
        onView={(nextYear, nextMonth) => {
          setView([nextYear, nextMonth])
        }}
      />
      <div className="flex flex-wrap gap-item">
        <Button
          variant="secondary"
          onClick={() => {
            onPick(monthOf(shownDay))
          }}
        >
          Whole month
        </Button>
        <Button
          variant="secondary"
          onClick={() => {
            onPick(yearOf(shownDay))
          }}
        >
          Whole year
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        Shift and Enter, or a shift-click, picks the days from the first one shown to the one
        chosen.
      </p>
    </section>
  )
}

// PeriodSteps move to the period before or after the one shown, by its
// length, and to today; Later is off once the period would start after today.
export function PeriodSteps({ period, today, onPick }: PeriodPickerProps) {
  const later = shiftPeriod(period, 1)

  return (
    <div className="flex flex-wrap gap-item">
      <Button
        variant="secondary"
        onClick={() => {
          onPick(shiftPeriod(period, -1))
        }}
      >
        Earlier
      </Button>
      <Button
        variant="secondary"
        aria-disabled={later.from > today}
        onClick={() => {
          if (later.from <= today) {
            onPick(later)
          }
        }}
      >
        Later
      </Button>
      <Button
        variant="secondary"
        onClick={() => {
          onPick({ from: today, to: today })
        }}
      >
        Today
      </Button>
    </div>
  )
}

interface MonthSelectsProps {
  year: number
  month: number
  thisYear: number
  onView: (year: number, month: number) => void
}

// MonthSelects choose the month the calendar shows, by year and month. The
// years offered run from a few before the year shown up to this one.
function MonthSelects({ year, month, thisYear, onView }: MonthSelectsProps) {
  const ids = { year: useId(), month: useId() }

  return (
    <div className="flex flex-wrap items-end gap-item">
      <label htmlFor={ids.year} className="flex flex-col gap-tight text-sm text-muted-foreground">
        Year
        <Select
          size="sm"
          id={ids.year}
          value={year}
          onChange={(event) => {
            onView(Number(event.target.value), month)
          }}
        >
          {yearsAround(year, thisYear).map((each) => (
            <option key={each} value={each}>
              {each}
            </option>
          ))}
        </Select>
      </label>
      <label htmlFor={ids.month} className="flex flex-col gap-tight text-sm text-muted-foreground">
        Month
        <Select
          size="sm"
          id={ids.month}
          value={month}
          onChange={(event) => {
            onView(year, Number(event.target.value))
          }}
        >
          {monthNames.map((name, index) => (
            <option key={name} value={index + 1}>
              {name}
            </option>
          ))}
        </Select>
      </label>
    </div>
  )
}

// yearsAround is the years the Year select offers: some before the year
// shown, through this year.
function yearsAround(year: number, thisYear: number): number[] {
  const first = Math.min(year, thisYear) - yearsBefore

  return Array.from({ length: Math.max(year, thisYear) - first + 1 }, (_, index) => first + index)
}
