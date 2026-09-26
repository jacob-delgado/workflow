import { announce as postAnnounce, getAnnouncement } from '@/api/generated'
import type { Announcement } from '@/api/generated/types.gen.ts'

// previewAnnouncement composes the announcement without posting it, for the
// confirm step to show. Under VITE_MOCK it returns a canned preview.
export async function previewAnnouncement(): Promise<Announcement> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return {
      text: 'octocat opened a pull request: redact tokens before they reach the request log\nhttps://example.com/pull/42 · PROJ-412',
      channel: '#dev-workflow',
    }
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
// message is safe to show. Under VITE_MOCK it answers with the canned preview,
// posted to the channel asked for.
export async function announce(channel: string, previewed: string): Promise<Announcement> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const preview = await previewAnnouncement()

    return { ...preview, channel }
  }

  const result = await postAnnounce({ body: { channel, text: previewed }, throwOnError: true })

  return result.data
}
