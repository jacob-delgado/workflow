import { linkBranchIssue, previewBranchIssue, unlinkBranchIssue } from '@/api/generated'
import type { Branch, BranchIssuePreview } from '@/api/generated/types.gen.ts'

// previewLink is what linking the branch to the issue key names would write in
// its pull request's description. Under VITE_MOCK the mockup's branch has no
// pull request, so nothing would change.
export async function previewLink(key: string): Promise<BranchIssuePreview> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return { key: key.replace(/^#/, ''), pull: 0, body: '', changes: false }
  }

  const result = await previewBranchIssue({ query: { key }, throwOnError: true })

  return result.data
}

// linkIssue links the branch to the issue key names, and with updatePull adds
// the issue's line to its pull request's description. A refusal throws the API
// error, whose message is safe to show. Under VITE_MOCK it answers with the
// mockup's branch, linked.
export async function linkIssue(key: string, updatePull: boolean): Promise<Branch> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return { ...mockSnapshot.branch, issue_link: key.replace(/^#/, '') }
  }

  const result = await linkBranchIssue({
    body: { key, update_pull: updatePull },
    throwOnError: true,
  })

  return result.data
}

// unlinkIssue forgets the issue the branch was linked to.
export async function unlinkIssue(): Promise<Branch> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return { ...mockSnapshot.branch, issue_link: '' }
  }

  const result = await unlinkBranchIssue({ throwOnError: true })

  return result.data
}
