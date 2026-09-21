import { getPullRequestDraft, openPullRequest } from '@/api/generated'
import type { OpenPullRequestRequest, PullRequestDraft } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and drops the SDK call from a production build's mock path, while
// tests can still stub the module.

// previewPullRequest composes the pull request that would be opened for the
// branch, for the form to start from. Under VITE_MOCK it returns a canned draft.
export async function previewPullRequest(): Promise<PullRequestDraft> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return {
      title: 'fix: redact tokens before they reach the request log',
      body: '## Commits\n\n- fix: redact tokens before they reach the request log\n\nPROJ-412',
      base: 'main',
      head: 'fix/PROJ-412',
      draft: false,
      needs_push: true,
    }
  }

  const result = await getPullRequestDraft({ throwOnError: true })

  return result.data
}

// openPr opens the pull request, pushing the branch first when needed. On
// success the event stream reflects the new pull request, so there is nothing to
// return; a refusal — nothing to open, a failed push or open — throws the API
// error, whose message is safe to show. Under VITE_MOCK it is a no-op.
export async function openPr(request: OpenPullRequestRequest): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await openPullRequest({ body: request, throwOnError: true })
}
