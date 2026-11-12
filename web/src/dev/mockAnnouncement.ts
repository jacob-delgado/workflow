import type {
  Announcement,
  OpenedPullRequest,
  QueuedAnnouncement,
} from '@/api/generated/types.gen.ts'
import { mockConfig } from './mockConfig.ts'
import { mockSnapshot } from './mockSnapshot.ts'

const current = mockSnapshot.branches.find((branch) => branch.current)
const issue = mockSnapshot.issues.issues.find((listed) => listed.key === current?.issue_key)
const pull = mockSnapshot.review.pull

// builtInText is the Slack announcement the server composes when the
// configuration names no template, as the mock's does not: the lead sentence
// linking the pull request, then the issue linked to its page and its summary
// (internal/messaging's defaultText). Every value is the mock's own, with
// nothing in it for Slack to read as markup.
function builtInText(): string {
  const key = current?.issue_key ?? ''
  const issueURL = `${mockConfig.jira.base_url ?? ''}/browse/${key}`
  const lead = `${mockSnapshot.messaging.author} opened a pull request: <${pull?.url ?? ''}|${pull?.title ?? ''}>`

  return `${lead}\n<${issueURL}|${key}> ${issue?.summary ?? ''}`
}

// mockAnnouncement is the announcement `task web:mockup` previews and posts:
// the built-in message for the mock pull request, as the server composes it
// from the branch, its issue and its pull. Dev-only, and code-split out of a
// production build.
export const mockAnnouncement: Announcement = {
  text: builtInText(),
  channel: mockSnapshot.messaging.channel,
  ci: mockSnapshot.review.ci?.state,
  // The mock pull request's CI has not passed, so the preview offers to wait.
  can_wait_for_ci: mockSnapshot.review.ci?.state !== 'passed',
}

// held is the announcement the mockup holds until CI passes, as the server
// holds one, which every frame carries once one was held; the mock's CI never
// passes, so it waits until it is stopped or announced now instead.
let held: QueuedAnnouncement | undefined

// mockHeld is the announcement the mockup holds, as a frame shows it.
export function mockHeld(): QueuedAnnouncement | undefined {
  return held
}

// mockHold holds the announcement of pull request number to channel until
// CI passes, afresh with the next id, armed as it opened when automatic says
// so, and answers it as held.
export function mockHold(number: number, channel: string, automatic = false): QueuedAnnouncement {
  held = {
    id: (held?.id ?? 0) + 1,
    state: 'waiting',
    waiting_on: 'ci',
    channel,
    pull: number,
    ...(mockAnnouncement.ci === undefined ? {} : { ci: mockAnnouncement.ci }),
    ...(automatic ? { automatic } : {}),
  }

  return held
}

// mockAnnouncedNow settles a waiting held announcement as announced, one
// asked for now having gone out in its place.
export function mockAnnouncedNow(): void {
  if (held?.state === 'waiting') {
    held = settled(held, 'announced')
  }
}

// mockStop stops the waiting held announcement, reporting whether one was
// waiting to stop; with none, nothing changes.
export function mockStop(): boolean {
  if (held?.state !== 'waiting') {
    return false
  }

  held = {
    ...settled(held, 'stopped'),
    reason: 'the announcement was stopped before it was posted',
  }

  return true
}

// settled is held at state, waiting no more.
function settled(
  waiting: QueuedAnnouncement,
  state: QueuedAnnouncement['state'],
): QueuedAnnouncement {
  const { id, channel, pull, automatic } = waiting

  return { id, state, channel, pull, ...(automatic === undefined ? {} : { automatic }) }
}

// mockArmedOn is what opening pull request number says of its announcement:
// armed to post once CI passes, which the mockup then holds, or, for a draft,
// why it is not.
export function mockArmedOn(
  number: number,
  draft: boolean,
): Pick<OpenedPullRequest, 'announcement' | 'not_armed'> {
  return draft
    ? { not_armed: 'a draft is not announced; announce it once it is ready' }
    : { announcement: mockHold(number, mockSnapshot.messaging.channel, true) }
}
