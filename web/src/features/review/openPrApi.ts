import { getPullRequestDraft, openPullRequest } from '@/api/generated'
import type {
  OpenedPullRequest,
  OpenPullRequestRequest,
  PullRequestDraft,
} from '@/api/generated/types.gen.ts'

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

// openPr opens the pull request, pushing the branch first when needed, and
// returns it with a warning when its reviewers, assignees or labels could not
// all be added, and what can be offered next. The event stream reflects the new
// pull request too. A refusal — nothing to open, a failed push or open — throws
// the API error, whose message is safe to show. Under VITE_MOCK it answers with
// a canned pull request that offers both follow-ups.
export async function openPr(request: OpenPullRequestRequest): Promise<OpenedPullRequest> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return {
      pull: {
        number: 42,
        url: 'https://example.com/pull/42',
        title: request.title,
        state: 'open',
        draft: request.draft ?? false,
        approvals: 0,
        changes_requested: false,
        mergeable: 'unknown',
      },
      follow_ups: [
        { action: 'link', issue_key: 'PROJ-412' },
        { action: 'transition', issue_key: 'PROJ-412', status: 'In Review' },
      ],
    }
  }

  const result = await openPullRequest({ body: request, throwOnError: true })

  return result.data
}
