import { useQuery } from '@tanstack/react-query'
import { postActivity } from '@/api/generated'
import { getActivityOptions } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { Activity, ActivityPost } from '@/api/generated/types.gen.ts'
import { freshFor } from '@/queryClient.ts'
import type { CivilDate, Period } from './civilDate.ts'

// useActivity reads back what you did over period, or the previous working day
// the server picks when period is null. What was done changes as you work, so
// the read is not kept fresh between visits; a failed one is not retried on its
// own, since each source has already said why it failed.
export function useActivity(period: Period | null) {
  return useQuery({
    ...getActivityOptions(period === null ? {} : { query: { from: period.from, to: period.to } }),
    staleTime: 0,
    retry: false,
  })
}

// MonthDone is what a month's read says was done: how many things on each day
// that saw any, and whether every source could say all of it. A source that
// failed, or had more than the month shows, may leave a day short.
export interface MonthDone {
  amounts: ReadonlyMap<CivilDate, number>
  whole: boolean
}

// MonthRead is the calendar's read of the month it shows: waiting on the
// server — or never asked, for a month that starts after today, in which
// nothing was done yet — refused, or read.
export type MonthRead =
  { state: 'waiting' } | { state: 'refused' } | { state: 'read'; done: MonthDone }

// useMonthRead reads what was done over month, for the calendar to say of each
// day. It is a read of its own, tagged for the calendar, never the period's
// even when the period is the month, so the calendar reading a month again can
// never set the period's read waiting or refused beneath it. It keeps its own
// freshness, so paging back to a month shown moments ago reads it no sooner
// than a section opened again would (freshFor).
//
// Trade-off TRADE-32: every month the calendar shows is read back from every
// source, on top of the period's own read.
export function useMonthRead(month: Period, today: CivilDate): MonthRead {
  const options = getActivityOptions({ query: { from: month.from, to: month.to } })
  const read = useQuery({
    ...options,
    queryKey: [{ ...options.queryKey[0], tags: ['calendar'] }],
    staleTime: freshFor,
    retry: false,
    enabled: month.from <= today,
  })

  if (read.isError) {
    return { state: 'refused' }
  }

  return read.isSuccess ? { state: 'read', done: monthDone(read.data) } : { state: 'waiting' }
}

// monthDone is how many things were done on each day an answer holds, and
// whether every source said all of it.
function monthDone(answer: Activity): MonthDone {
  const days = answer.years.flatMap((year) => year.months.flatMap((month) => month.days))

  return {
    amounts: new Map(
      days.map((day) => [day.date, day.hours.reduce((sum, hour) => sum + hour.items.length, 0)]),
    ),
    whole: answer.sources.every((source) => source.state !== 'failed' && !source.truncated),
  }
}

// postSummary posts text — the summary of the days from and to, as the server
// wrote it or as it was edited — to channel, or the configured one or a
// webhook's own when channel is empty. The server renders it for the service
// and keeps nothing of it; it answers where it went. A refusal throws the API
// error, whose message is safe to show.
export async function postSummary(
  from: string,
  to: string,
  text: string,
  channel: string,
): Promise<ActivityPost> {
  const body = { from, to, text, ...(channel === '' ? {} : { channel }) }
  const result = await postActivity({ body, throwOnError: true })

  return result.data
}
