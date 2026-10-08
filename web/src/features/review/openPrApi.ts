import { getPullRequestDraft, openPullRequest } from '@/api/generated'
import type {
  OpenedPullRequest,
  OpenPullRequestRequest,
  PullRequestDraft,
} from '@/api/generated/types.gen.ts'

// previewPullRequest composes the pull request that would be opened for the
// branch, for the form to start from — its body from the template named, or
// the repository's first.
export async function previewPullRequest(template?: string): Promise<PullRequestDraft> {
  const query = template === undefined ? {} : { template }
  const result = await getPullRequestDraft({ query, throwOnError: true })

  return result.data
}

// openPr opens the pull request, pushing the branch first when needed, and
// returns it with a warning when its reviewers, assignees or labels could not
// all be added, and what can be offered next. The event stream reflects the new
// pull request too. A refusal — nothing to open, a failed push or open — throws
// the API error, whose message is safe to show.
export async function openPr(request: OpenPullRequestRequest): Promise<OpenedPullRequest> {
  const result = await openPullRequest({ body: request, throwOnError: true })

  return result.data
}
