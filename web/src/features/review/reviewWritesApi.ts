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

// Each write below throws the API error on a refusal, whose message is safe to
// show; the event stream shows what it did.

// readPullText reads the branch's open pull request's title and description
// afresh, for the editor.
export async function readPullText(): Promise<PullRequestText> {
  const result = await getPullRequestText({ throwOnError: true })

  return result.data
}

// editPull saves the pull request's title and description.
export async function editPull(title: string, body: string): Promise<PullRequest> {
  const result = await editPullRequest({ body: { title, body }, throwOnError: true })

  return result.data
}

// readMergeOffer reads the methods the repository permits a merge by, once the
// pull request can be merged.
export async function readMergeOffer(): Promise<MergeOffer> {
  const result = await getMergeMethods({ throwOnError: true })

  return result.data
}

// mergePull merges the pull request by method.
export async function mergePull(method: MergeMethod): Promise<PullRequest> {
  const result = await mergePullRequest({ body: { method }, throwOnError: true })

  return result.data
}

// finishBranch finishes the merged branch and answers the base, now checked
// out.
export async function finishBranch(): Promise<Branch> {
  const result = await postFinish({ throwOnError: true })

  return result.data
}

// rerunChecks re-runs the failed CI, and says whether anything was re-run.
export async function rerunChecks(): Promise<Rerun> {
  const result = await postRerun({ throwOnError: true })

  return result.data
}
