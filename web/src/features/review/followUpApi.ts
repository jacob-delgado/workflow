import { linkPullRequest, transitionIssue } from '@/api/generated'

// linkOnIssue records the checked-out branch's pull request as a link on the
// issue it names. The server finds the pull request; only the issue is sent. A
// refusal — a branch that names another issue, nothing to link, a tracker that
// refuses — throws the API error, whose message is safe to show.
export async function linkOnIssue(issueKey: string): Promise<void> {
  await linkPullRequest({ path: { key: issueKey }, throwOnError: true })
}

// moveToReview moves the issue to the configured review status, and only
// there. A move Jira wants fields for, or does not offer, throws the API error.
export async function moveToReview(issueKey: string): Promise<void> {
  await transitionIssue({ path: { key: issueKey }, throwOnError: true })
}
