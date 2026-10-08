import {
  announce as postAnnounce,
  cancelQueuedAnnouncement,
  getAnnouncement,
} from '@/api/generated'
import type {
  Announcement,
  AnnounceMentions,
  AnnounceRequest,
  QueuedAnnouncement,
} from '@/api/generated/types.gen.ts'

// previewAnnouncement composes the announcement without posting it, for the
// confirm step to show, with whom it proposes to tag when the server has a
// Slack user token — an owner not yet linked checked against channel's
// members, the configured channel's when empty.
export async function previewAnnouncement(channel = ''): Promise<Announcement> {
  const query = channel === '' ? {} : { channel }
  const result = await getAnnouncement({ query, throwOnError: true })

  return result.data
}

// announce posts previewed — the announcement as its preview showed it — to
// channel on the configured service, and returns it as posted, with the channel
// it went to, for the panel to say where, and the server's warning when the
// store could not remember it, so a later session may offer it again. The server posts that text or
// nothing: an announcement that reads differently by the time of the post (CI
// turned red, the pull request merged) is refused, to be previewed again. A
// refusal — that, no pull request, a failed post — throws the API error, whose
// message is safe to show. Given mentions — the people the preview showed
// tagged and the user groups checked, for an announcement that tags — the
// server tags the linked code owners and the groups on a line after the
// text, refusing the post when the people linked are no longer the ones
// shown. Given edited, the text the preview was edited to, that is posted in
// place of previewed, which the server still checks against the announcement
// composed now.
export async function announce(
  channel: string,
  previewed: string,
  mentions?: AnnounceMentions,
  edited?: string,
): Promise<Announcement> {
  const result = await postAnnounce({
    body: announceBody(channel, previewed, mentions, edited),
    throwOnError: true,
  })

  return posted(result.data)
}

// Held says whether an announcement asked to wait for CI is held, or went at
// once because its CI had passed by then, where it goes, and the server's
// warning when it went but could not be remembered.
export interface Held {
  held: boolean
  channel: string
  warning?: string | undefined
}

// announceWhenCIPasses asks the server to hold the previewed announcement
// until the pull request's CI passes — posting it at once if it has — as
// announce does with the same arguments. The server holds it while it runs,
// and the stream says how it stands.
export async function announceWhenCIPasses(
  channel: string,
  previewed: string,
  mentions?: AnnounceMentions,
  edited?: string,
): Promise<Held> {
  const result = await postAnnounce({
    body: { ...announceBody(channel, previewed, mentions, edited), when: 'ci_passes' },
    throwOnError: true,
  })

  return {
    held: result.response.status === 202,
    channel: result.data.channel,
    warning: result.data.warning,
  }
}

// stopWaiting drops the announcement held for CI, unposted.
export async function stopWaiting(): Promise<void> {
  await cancelQueuedAnnouncement({ throwOnError: true })
}

// announceBody is a post's request: the channel, the previewed text, and the
// mentions and the edit when there are any.
function announceBody(
  channel: string,
  previewed: string,
  mentions?: AnnounceMentions,
  edited?: string,
): AnnounceRequest {
  return {
    channel,
    text: previewed,
    ...(mentions === undefined ? {} : { mentions }),
    ...(edited === undefined ? {} : { edited_text: edited }),
  }
}

// posted is the announcement a post answered: a post made now always answers
// the announcement, never one held.
function posted(answer: Announcement | QueuedAnnouncement): Announcement {
  if (!('text' in answer)) {
    throw new Error('the server held an announcement it was asked to post')
  }

  return answer
}
