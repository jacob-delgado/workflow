import { linkPullRequest, transitionIssue } from '@/api/generated'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and drops the SDK call from a production build's mock path, while
// tests can still stub the module.

// linkOnIssue records the checked-out branch's pull request as a link on the
// issue it names. The server finds the pull request; only the issue is sent. A
// refusal — a branch that names another issue, nothing to link, a tracker that
// refuses — throws the API error, whose message is safe to show. Under
// VITE_MOCK it is a no-op.
export async function linkOnIssue(issueKey: string): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await linkPullRequest({ path: { key: issueKey }, throwOnError: true })
}

// moveToReview moves the issue to the configured review status, and only
// there. A move Jira wants fields for, or does not offer, throws the API error.
// Under VITE_MOCK it is a no-op.
export async function moveToReview(issueKey: string): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await transitionIssue({ path: { key: issueKey }, throwOnError: true })
}
