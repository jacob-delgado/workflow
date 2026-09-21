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

// announce posts the composed announcement to Slack. On success there is nothing
// to return; a refusal — no pull request, a failed post — throws the API error,
// whose message is safe to show. Under VITE_MOCK it is a no-op.
export async function announce(channel: string): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await postAnnounce({ body: { channel }, throwOnError: true })
}
