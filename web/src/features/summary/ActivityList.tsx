import type {
  ActivityDay,
  ActivityHour,
  ActivityItem,
  ActivityMonth,
  ActivityYear,
} from '@/api/generated/types.gen.ts'
import { headingDay } from '@/lib/dates.ts'
import { cn } from '@/lib/utils.ts'

// sourceHues color what was done in the hue of the system the terminal's spine
// colors it: a commit in git's, a task in Taskwarrior's, an issue in Jira's,
// and a pull or merge request in the forge's. The verb and what it was done to
// say it in words too, so the hue is never the only sign.
const sourceHues: Record<ActivityItem['source'], string> = {
  git: 'text-git',
  tasks: 'text-taskwarrior',
  jira: 'text-jira',
  forge: 'text-forge',
}

// ActivityList is what was done, oldest first: a heading for each month and
// day, and each day a timeline whose left column is the hour and whose items
// hang off it, each opening with what it is in its system's hue. A period of
// one day is headed above the list already, so its timeline stands alone.
export function ActivityList({ years, oneDay }: { years: ActivityYear[]; oneDay: boolean }) {
  const only = years[0]?.months[0]?.days[0]
  if (oneDay && only !== undefined) {
    return <DayTimeline day={only} />
  }

  return (
    <div className="flex flex-col gap-section">
      {years.flatMap((year) =>
        year.months.map((month) => (
          <MonthSection
            key={`${String(year.year)}-${String(month.month)}`}
            year={year.year}
            month={month}
          />
        )),
      )}
    </div>
  )
}

// MonthSection is a month's heading and its days.
function MonthSection({ year, month }: { year: number; month: ActivityMonth }) {
  return (
    <section className="flex flex-col gap-group">
      <h3 className="text-base font-semibold">
        {month.name} {year}
      </h3>
      {month.days.map((day) => (
        <DaySection key={day.date} day={day} />
      ))}
    </section>
  )
}

// DaySection is a day's heading and its timeline, an hour to a row.
function DaySection({ day }: { day: ActivityDay }) {
  return (
    <section className="flex flex-col gap-item">
      <h4 className="text-sm font-semibold text-foreground">
        {day.weekday} {Number(day.date.slice(8))}
      </h4>
      <DayTimeline day={day} />
    </section>
  )
}

// DayTimeline is a day's timeline, an hour to a row, named for its day.
function DayTimeline({ day }: { day: ActivityDay }) {
  return (
    <ol
      aria-label={headingDay(day.date)}
      className="flex flex-col gap-group border-l border-border"
    >
      {day.hours.map((hour) => (
        <HourRow key={hour.label} hour={hour} />
      ))}
    </ol>
  )
}

// HourRow is an hour on the timeline's left and its items beside it.
function HourRow({ hour }: { hour: ActivityHour }) {
  return (
    <li className="grid grid-cols-[5.5rem_minmax(0,1fr)] gap-x-group">
      <h5 className="-ml-px border-l-2 border-ring pl-3 text-sm font-normal text-muted-foreground tabular-nums">
        {hour.label}
      </h5>
      <ul className="flex min-w-0 flex-col gap-tight">
        {/* Two worklogs on one issue can share a second, so the place in the
            hour, which an answer never reorders, tells them apart. */}
        {hour.items.map((item, place) => (
          <ActivityLine key={`${item.at}-${item.ref}-${item.verb}-${String(place)}`} item={item} />
        ))}
      </ul>
    </li>
  )
}

// ActivityLine is one thing done: the verb in its source's hue, what it was
// done to — a link where there is one — and its title.
function ActivityLine({ item }: { item: ActivityItem }) {
  return (
    <li className="flex flex-wrap items-baseline gap-x-item text-sm break-words">
      <span className={cn('font-medium', sourceHues[item.source])}>{item.verb}</span>
      {item.url === '' ? (
        <code className="font-mono text-foreground">{item.ref}</code>
      ) : (
        <a
          href={item.url}
          target="_blank"
          rel="noreferrer"
          className="font-mono text-foreground underline underline-offset-2 hover:text-primary focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          {item.ref} <span className="sr-only">(opens in a new tab)</span>
        </a>
      )}
      <span className="min-w-0 text-foreground">{item.title}</span>
    </li>
  )
}
