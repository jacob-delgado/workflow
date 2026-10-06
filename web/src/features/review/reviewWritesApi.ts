import {
  editPullRequest,
  finishBranch as postFinish,
  getMergeMethods,
  getPullRequestText,
  mergePullRequest,
  rerunChecks as postRerun,
} from '@/api/generated'
import type {
  Branch,
  MergeMethod,
  MergeOffer,
  PullRequest,
  PullRequestText,
  Rerun,
} from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and drops the SDK call from a production build's mock path, while
// tests can still stub the module. Each write below throws the API error on a
// refusal, whose message is safe to show; the event stream shows what it did.

// readPullText reads the branch's open pull request's title and description
// afresh, for the editor.
export async function readPullText(): Promise<PullRequestText> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return {
      title: 'fix: redact tokens in the request log',
      body: '## Why\n\nTokens reached the log.',
    }
  }

  const result = await getPullRequestText({ throwOnError: true })

  return result.data
}

// editPull saves the pull request's title and description.
export async function editPull(title: string, body: string): Promise<PullRequest> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return { ...(mockSnapshot.review.pull as PullRequest), title }
  }

  const result = await editPullRequest({ body: { title, body }, throwOnError: true })

  return result.data
}

// readMergeOffer reads the methods the repository permits a merge by, once the
// pull request can be merged.
export async function readMergeOffer(): Promise<MergeOffer> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return { pull: mockSnapshot.review.pull as PullRequest, methods: ['squash', 'merge'] }
  }

  const result = await getMergeMethods({ throwOnError: true })

  return result.data
}

// mergePull merges the pull request by method.
export async function mergePull(method: MergeMethod): Promise<PullRequest> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return { ...(mockSnapshot.review.pull as PullRequest), state: 'merged' }
  }

  const result = await mergePullRequest({ body: { method }, throwOnError: true })

  return result.data
}

// finishBranch finishes the merged branch and answers the base, now checked
// out.
export async function finishBranch(): Promise<Branch> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return { ...mockSnapshot.branch, name: 'main', commits: [] }
  }

  const result = await postFinish({ throwOnError: true })

  return result.data
}

// rerunChecks re-runs the failed CI, and says whether anything was re-run.
export async function rerunChecks(): Promise<Rerun> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return { reran: true }
  }

  const result = await postRerun({ throwOnError: true })

  return result.data
}
