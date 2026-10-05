import { useQuery } from '@tanstack/react-query'
import { getActivityOptions } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { Activity } from '@/api/generated/types.gen.ts'
import type { Period } from './civilDate.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build, while
// tests can still stub it at runtime.

// useActivity reads back what you did over period, or the previous working day
// the server picks when period is null. What was done changes as you work, so
// the read is not kept fresh between visits; a failed one is not retried on its
// own, since each source has already said why it failed. Under VITE_MOCK it
// serves the mockup's day, so the section is filled with no backend.
export function useActivity(period: Period | null) {
  const options = {
    ...getActivityOptions(period === null ? {} : { query: { from: period.from, to: period.to } }),
    staleTime: 0,
    retry: false,
  }

  return useQuery(
    import.meta.env.VITE_MOCK === 'true'
      ? {
          ...options,
          queryFn: async (): Promise<Activity> => {
            const { mockActivity } = await import('@/dev/mockActivity.ts')

            return mockActivity(period)
          },
        }
      : options,
  )
}
