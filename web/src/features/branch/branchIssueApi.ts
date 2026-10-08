import { linkBranchIssue, previewBranchIssue, unlinkBranchIssue } from '@/api/generated'
import type { Branch, BranchIssuePreview } from '@/api/generated/types.gen.ts'

// previewLink is what linking the branch to the issue key names would write in
// its pull request's description.
export async function previewLink(key: string): Promise<BranchIssuePreview> {
  const result = await previewBranchIssue({ query: { key }, throwOnError: true })

  return result.data
}

// linkIssue links the branch to the issue key names, and with updatePull adds
// the issue's line to its pull request's description. A refusal throws the API
// error, whose message is safe to show.
export async function linkIssue(key: string, updatePull: boolean): Promise<Branch> {
  const result = await linkBranchIssue({
    body: { key, update_pull: updatePull },
    throwOnError: true,
  })

  return result.data
}

// unlinkIssue forgets the issue the branch was linked to.
export async function unlinkIssue(): Promise<Branch> {
  const result = await unlinkBranchIssue({ throwOnError: true })

  return result.data
}
