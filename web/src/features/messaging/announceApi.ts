import { announce as postAnnounce, getAnnouncement } from '@/api/generated'
import type { Announcement, AnnounceMentions } from '@/api/generated/types.gen.ts'

// previewAnnouncement composes the announcement without posting it, for the
// confirm step to show, with whom it proposes to tag when the server has a
// Slack user token — an owner not yet linked checked against channel's
// members, the configured channel's when empty. Under VITE_MOCK it serves the
// fixture (code-split, dev-only): the mock template filled from the mock
// snapshot, and the mock Slack's tags.
export async function previewAnnouncement(channel = ''): Promise<Announcement> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockAnnouncement } = await import('@/dev/mockAnnouncement.ts')
    const { mockTagging } = await import('@/dev/mockSlack.ts')

    return { ...mockAnnouncement, tagging: mockTagging() }
  }

  const query = channel === '' ? {} : { channel }
  const result = await getAnnouncement({ query, throwOnError: true })

  return result.data
}

// announce posts previewed — the announcement as its preview showed it — to
// channel on the configured service, and returns it as posted, with the channel
// it went to, for the panel to say where. The server posts that text or
// nothing: an announcement that reads differently by the time of the post (CI
// turned red, the pull request merged) is refused, to be previewed again. A
// refusal — that, no pull request, a failed post — throws the API error, whose
// message is safe to show. Given mentions — the people the preview showed
// tagged and the user groups checked, for an announcement that tags — the
// server tags the linked code owners and the groups on a line after the
// text, refusing the post when the people linked are no longer the ones
// shown. Under VITE_MOCK it answers with the mock preview, posted to the
// channel asked for.
export async function announce(
  channel: string,
  previewed: string,
  mentions?: AnnounceMentions,
): Promise<Announcement> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const preview = await previewAnnouncement()

    return { ...preview, channel }
  }

  const tags = mentions === undefined ? {} : { mentions }
  const result = await postAnnounce({
    body: { channel, text: previewed, ...tags },
    throwOnError: true,
  })

  return result.data
}
