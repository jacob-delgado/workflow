import { useState } from 'react'
import type { Activity } from '@/api/generated/types.gen.ts'
import { apiErrorMessage } from '@/api/apiError.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { ActivityList } from './ActivityList.tsx'
import { spokenDate, type Period } from './civilDate.ts'
import { PeriodCalendar, PeriodSteps } from './PeriodPicker.tsx'
import { useActivity } from './summaryApi.ts'

// SummaryPanel is what you did over a period, read back from git, Taskwarrior,
// Jira and the forge: the previous working day until another is picked, a
// timeline of it beside the calendar it is picked in, what each source could
// not say, and the summary to copy as Markdown.
export function SummaryPanel() {
  const chosen = useUiStore((state) => state.summaryPeriod)
  const choose = useUiStore((state) => state.setSummaryPeriod)
  const read = useActivity(chosen)
  const last = useLastAnswer(read.data)

  if (last === undefined) {
    return <FirstRead read={read} />
  }

  const period = chosen ?? { from: last.from, to: last.to }

  // The steps, then what was done, then the calendar: on a narrow screen what
  // was done comes before the calendar it is picked in, and on a wide one the
  // steps and the calendar share the left column, in that same order. The last
  // answer keeps them drawn while another period is read, or refused, so a
  // pick never takes the calendar, or the focus in it, away.
  return (
    <div className="flex flex-col gap-section lg:grid lg:grid-cols-[18rem_minmax(0,1fr)] lg:grid-rows-[auto_1fr] lg:items-start lg:gap-x-section lg:gap-y-group">
      <div className="lg:col-start-1 lg:row-start-1">
        <PeriodSteps period={period} today={last.today} onPick={choose} />
      </div>
      <div className="lg:col-start-2 lg:row-span-2 lg:row-start-1">
        <Shown read={read} period={period} />
      </div>
      <div className="lg:col-start-1 lg:row-start-2">
        <PeriodCalendar
          key={period.to.slice(0, 7)}
          period={period}
          today={last.today}
          onPick={choose}
        />
      </div>
    </div>
  )
}

// useLastAnswer is the latest activity the server answered with, kept while
// another period is read or after one is refused.
function useLastAnswer(answer: Activity | undefined): Activity | undefined {
  const [last, setLast] = useState(answer)
  if (answer !== undefined && answer !== last) {
    setLast(answer)

    return answer
  }

  return last
}

type ActivityRead = ReturnType<typeof useActivity>

// FirstRead is the section before the server has answered once: reading, or
// why it could not, with another try.
function FirstRead({ read }: { read: ActivityRead }) {
  return read.isError ? <Refused read={read} /> : <Reading />
}

// Refused is why the period could not be read, with another try.
function Refused({ read }: { read: ActivityRead }) {
  return (
    <div className="flex flex-col items-start gap-item">
      <p className="text-sm text-foreground">
        {apiErrorMessage(read.error, 'What you did could not be read.')}
      </p>
      <Button
        variant="secondary"
        onClick={() => {
          void read.refetch()
        }}
      >
        Retry
      </Button>
    </div>
  )
}

// Reading says a period is being read.
function Reading() {
  return (
    <p role="status" className="text-sm text-muted-foreground">
      Reading what you did…
    </p>
  )
}

interface ShownProps {
  read: ActivityRead
  period: Period
}

// Shown is the period named, then why it could not be read, that it is being
// read, or what was done in it.
function Shown({ read, period }: ShownProps) {
  return (
    <div className="flex min-w-0 flex-col gap-group">
      <h2 className="text-lg font-semibold">
        {period.from === period.to
          ? spokenDate(period.from)
          : `${spokenDate(period.from)} to ${spokenDate(period.to)}`}
      </h2>
      {read.isError ? <Refused read={read} /> : null}
      {!read.isError && read.data === undefined ? <Reading /> : null}
      {read.data === undefined ? null : (
        <Done activity={read.data} period={period} reading={read.isFetching} />
      )}
    </div>
  )
}

interface DoneProps {
  activity: Activity
  period: Period
  reading: boolean
}

// Done is the summary to copy, what each source could not say, and the
// period's timeline or that nothing was done.
function Done({ activity, period, reading }: DoneProps) {
  const outcome = useOutcome()
  const written = period.from === period.to ? period.from : `${period.from} to ${period.to}`
  const empty = activity.years.length === 0
  const copy = useAsyncAction((text: string) => navigator.clipboard.writeText(text), {
    fallback: 'The summary could not be copied.',
    done: () => `Copied the summary of ${written}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <>
      <div>
        <Button
          variant="secondary"
          aria-disabled={empty}
          onClick={() => {
            if (!empty) {
              void copy.run(activity.text)
            }
          }}
        >
          Copy as Markdown
        </Button>
      </div>
      <OutcomeLine said={outcome.said} />
      {copy.state === 'error' ? <p className="text-sm text-foreground">{copy.error}</p> : null}
      {reading ? <Reading /> : null}
      {activity.sources
        .filter((source) => source.failed || source.truncated)
        .map((source) => (
          <p key={source.source} className="text-sm text-foreground">
            {source.failed
              ? `${source.name} could not be read: ${source.detail}`
              : `${source.name} had more than this shows.`}
          </p>
        ))}
      {empty ? (
        <p className="text-sm text-muted-foreground">
          Nothing done in this period. Pick another day or range.
        </p>
      ) : (
        <ActivityList years={activity.years} />
      )}
    </>
  )
}
