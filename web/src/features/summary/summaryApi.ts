import { useQuery } from '@tanstack/react-query'
import { postActivity } from '@/api/generated'
import { getActivityOptions } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { ActivityPost } from '@/api/generated/types.gen.ts'
import type { Period } from './civilDate.ts'

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
