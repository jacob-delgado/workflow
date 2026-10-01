import { getCheckLog } from '@/api/generated'
import type { JobLog } from '@/api/generated/types.gen.ts'

// readCheckLog reads the end of a failed check's log, on demand: a refusal
// throws the API error, whose message is safe to show. Under VITE_MOCK it
// answers with a short failing log, so the mockup can show one.
export async function readCheckLog(id: string): Promise<JobLog> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return {
      text: '--- FAIL: TestRetry (0.01s)\n    retry_test.go:41: got 4, want 3\nFAIL',
      truncated: true,
    }
  }

  const result = await getCheckLog({ path: { id }, throwOnError: true })

  return result.data
}
