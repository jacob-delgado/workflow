import { useQuery } from '@tanstack/react-query'
import { listReviewsOptions } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { ReviewQueue } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build, while
// tests can still stub it at runtime.

// How long a read of the review queue is taken as current. The queue is a
// search of the forge across repositories, under its rate limits, and nothing
// pushes a newer one: past this, opening the section reads it again, and
// Refresh reads it at once. Nothing polls — a queue nobody is looking at is not
// worth a request.
const queueFreshFor = 60_000

// useReviewQueue reads the pull requests on the forge that wait on your review,
// the longest-waiting first. A failed read is not retried on its own: the
// server has already said what went wrong, and asking a forge that is limiting
// requests again only spends more of the limit — Retry is the user's to press.
// Under VITE_MOCK it serves the mockup's queue, so the section is filled with
// no backend.
export function useReviewQueue() {
  const options = { ...listReviewsOptions(), staleTime: queueFreshFor, retry: false }

  return useQuery(
    import.meta.env.VITE_MOCK === 'true'
      ? {
          ...options,
          queryFn: async (): Promise<ReviewQueue> => {
            const { mockReviewQueue } = await import('@/dev/mockReviews.ts')

            return mockReviewQueue()
          },
        }
      : options,
  )
}
