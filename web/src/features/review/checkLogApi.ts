import { getCheckLog } from '@/api/generated'
import type { JobLog } from '@/api/generated/types.gen.ts'

// readCheckLog reads the end of a failed check's log, on demand: a refusal
// throws the API error, whose message is safe to show.
export async function readCheckLog(id: string): Promise<JobLog> {
  const result = await getCheckLog({ path: { id }, throwOnError: true })

  return result.data
}
