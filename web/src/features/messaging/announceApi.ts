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

// announce posts the composed announcement to the configured service and
// returns it as posted, with the channel it went to, for the panel to say
// where. A refusal — no pull request, a failed post — throws the API error,
// whose message is safe to show. Under VITE_MOCK it answers with the canned
// preview, posted to the channel asked for.
export async function announce(channel: string): Promise<Announcement> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const preview = await previewAnnouncement()

    return { ...preview, channel }
  }

  const result = await postAnnounce({ body: { channel }, throwOnError: true })

  return result.data
}
