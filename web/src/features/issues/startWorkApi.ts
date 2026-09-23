import { createBranch } from '@/api/generated'

// startWork creates and switches to a branch for a not-started issue — the write
// half of starting work on it. On success the event stream reflects the new
// branch; a refusal (a branch already exists) throws the API error, whose
// message is safe to show through apiErrorMessage. Under VITE_MOCK it is a
// no-op, so the mockup's button is inert.
export async function startWork(issueKey: string): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await createBranch({ body: { issue_key: issueKey }, throwOnError: true })
}
