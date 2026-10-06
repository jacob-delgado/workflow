import { useEffect, useRef, useState, type KeyboardEvent, type RefObject } from 'react'
import { headingDay, monthHeading } from '@/lib/dates.ts'
import { cn } from '@/lib/utils.ts'
import {
  addDays,
  addMonths,
  monthWeeks,
  yearMonth,
  type CivilDate,
  type Period,
} from './civilDate.ts'

const weekdays = [
  ['Mo', 'Monday'],
  ['Tu', 'Tuesday'],
  ['We', 'Wednesday'],
  ['Th', 'Thursday'],
  ['Fr', 'Friday'],
  ['Sa', 'Saturday'],
  ['Su', 'Sunday'],
] as const

// keySteps is how far each key moves the day in focus: a day, a week, or a
// month, as the WAI-ARIA date grid moves it.
const keySteps: Record<string, (date: CivilDate) => CivilDate> = {
  ArrowLeft: (date) => addDays(date, -1),
  ArrowRight: (date) => addDays(date, 1),
  ArrowUp: (date) => addDays(date, -7),
  ArrowDown: (date) => addDays(date, 7),
  PageUp: (date) => addMonths(date, -1),
  PageDown: (date) => addMonths(date, 1),
  Home: (date) => addDays(date, -weekdayOf(date)),
  End: (date) => addDays(date, 6 - weekdayOf(date)),
}

interface MonthGridProps {
  year: number
  month: number
  period: Period
  onPick: (period: Period) => void
  // onView hears a key move focus into another month, so the selects above
  // the grid follow it.
  onView?: (year: number, month: number) => void
}

// MonthGrid is a month of days, a week to a row starting on Monday, in which
// one day at a time is reached by Tab and the arrows move between them. Enter
// or a click picks a day, and with Shift picks the days from the period's first
// to it. The days of the period shown are selected.
export function MonthGrid({ year, month, period, onPick, onView }: MonthGridProps) {
  const [focused, setFocused] = useState<CivilDate>(() => inMonth(period.to, year, month))
  const moved = useRef(false)
  const cells = useRef(new Map<CivilDate, HTMLDivElement>())
  const shown = inMonth(focused, year, month)

  useEffect(() => {
    if (moved.current) {
      moved.current = false
      cells.current.get(shown)?.focus()
    }
  }, [shown])

  const pick = (date: CivilDate, extend: boolean) => {
    setFocused(date)
    onPick(extend ? rangeTo(period, date) : { from: date, to: date })
  }

  const onKey = (event: KeyboardEvent<HTMLDivElement>) => {
    const step = keySteps[event.key]
    if (step !== undefined) {
      event.preventDefault()
      const next = step(shown)
      moved.current = true
      setFocused(next)
      const [nextYear, nextMonth] = yearMonth(next)
      if (nextYear !== year || nextMonth !== month) {
        onView?.(nextYear, nextMonth)
      }
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      pick(shown, event.shiftKey)
    }
  }

  return (
    <div
      role="grid"
      aria-label={monthHeading(year, month)}
      className="flex flex-col gap-tight text-sm"
    >
      <div role="row" className="grid grid-cols-7">
        {weekdays.map(([short, long]) => (
          <div
            key={short}
            role="columnheader"
            aria-label={long}
            className="pb-1 text-center text-xs text-muted-foreground"
          >
            {short}
          </div>
        ))}
      </div>
      {monthWeeks(year, month).map((week) => (
        <div
          key={week.find((day) => day !== null) ?? ''}
          role="row"
          className="grid grid-cols-7 gap-tight"
        >
          {week.map((day, index) =>
            day === null ? (
              <div key={`blank-${String(index)}`} role="gridcell" aria-hidden />
            ) : (
              <DayCell
                key={day}
                day={day}
                inPeriod={day >= period.from && day <= period.to}
                focusable={day === shown}
                cells={cells}
                onPick={pick}
                onKey={onKey}
              />
            ),
          )}
        </div>
      ))}
    </div>
  )
}

interface DayCellProps {
  day: CivilDate
  inPeriod: boolean
  focusable: boolean
  cells: RefObject<Map<CivilDate, HTMLDivElement>>
  onPick: (day: CivilDate, extend: boolean) => void
  onKey: (event: KeyboardEvent<HTMLDivElement>) => void
}

// DayCell is one day: selected when it is in the period, reached by Tab when
// it is the day in focus, and picked by a click or a key, with Shift for a
// range.
function DayCell({ day, inPeriod, focusable, cells, onPick, onKey }: DayCellProps) {
  return (
    <div
      ref={(cell) => {
        if (cell === null) {
          cells.current.delete(day)
        } else {
          cells.current.set(day, cell)
        }
      }}
      role="gridcell"
      aria-label={headingDay(day)}
      aria-selected={inPeriod}
      tabIndex={focusable ? 0 : -1}
      onClick={(event) => {
        onPick(day, event.shiftKey)
      }}
      onKeyDown={onKey}
      className={cn(
        'flex h-8 cursor-pointer items-center justify-center rounded-md tabular-nums focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
        inPeriod ? 'bg-primary text-primary-foreground' : 'text-foreground hover:bg-accent',
      )}
    >
      {Number(day.slice(8))}
    </div>
  )
}

// inMonth is date when it falls in the month shown, or that month's day of
// the same number, the last where it has none.
function inMonth(date: CivilDate, year: number, month: number): CivilDate {
  const [dateYear, dateMonth] = yearMonth(date)

  return addMonths(date, (year - dateYear) * 12 + (month - dateMonth))
}

// rangeTo is the days from the period's first to date, whichever comes first.
function rangeTo(period: Period, date: CivilDate): Period {
  return date < period.from ? { from: date, to: period.to } : { from: period.from, to: date }
}

// weekdayOf is a date's day of the week, Monday 0 through Sunday 6.
function weekdayOf(date: CivilDate): number {
  return (new Date(`${date}T12:00:00Z`).getUTCDay() + 6) % 7
}
