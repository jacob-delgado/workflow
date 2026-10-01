import { announce as postAnnounce, getAnnouncement } from '@/api/generated'
import type { Announcement } from '@/api/generated/types.gen.ts'

// previewAnnouncement composes the announcement without posting it, for the
// confirm step to show, with whom it proposes to tag when the server has a
// Slack user token. Under VITE_MOCK it serves the fixture (code-split,
// dev-only): the mock template filled from the mock snapshot, and the mock
// Slack's tags.
export async function previewAnnouncement(): Promise<Announcement> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockAnnouncement } = await import('@/dev/mockAnnouncement.ts')
    const { mockTagging } = await import('@/dev/mockSlack.ts')

    return { ...mockAnnouncement, tagging: mockTagging() }
  }

  const result = await getAnnouncement({ throwOnError: true })

  return result.data
}

// announce posts previewed — the announcement as its preview showed it — to
// channel on the configured service, and returns it as posted, with the channel
// it went to, for the panel to say where. The server posts that text or
// nothing: an announcement that reads differently by the time of the post (CI
// turned red, the pull request merged) is refused, to be previewed again. A
// refusal — that, no pull request, a failed post — throws the API error, whose
// message is safe to show. Given groups — the user groups checked, for an
// announcement that tags — the server tags them and the linked code owners
// on a line after the text. Under VITE_MOCK it answers with the mock
// preview, posted to the channel asked for.
export async function announce(
  channel: string,
  previewed: string,
  groups?: string[],
): Promise<Announcement> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const preview = await previewAnnouncement()

    return { ...preview, channel }
  }

  const mentions = groups === undefined ? {} : { mentions: { groups } }
  const result = await postAnnounce({
    body: { channel, text: previewed, ...mentions },
    throwOnError: true,
  })

  return result.data
}
