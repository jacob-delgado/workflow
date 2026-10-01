import { useQuery, useQueryClient } from '@tanstack/react-query'
import { cleanLocalData, getLocalData } from '@/api/generated'
import { getLocalDataQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { LocalData } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build, while
// tests can still stub it at runtime.

// CleanScope is how much a clean removes: the cache alone, or the kept
// associations too.
export type CleanScope = 'cache' | 'all'

// readLocalData reads the store's directory and its database files. Under
// VITE_MOCK it serves the mockup's store, as a clean there has left it.
async function readLocalData(signal?: AbortSignal): Promise<LocalData> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockLocalData } = await import('@/dev/mockLocalData.ts')

    return mockLocalData()
  }

  const { data } = await getLocalData({ signal, throwOnError: true })

  return data
}

// useLocalData reads the store's files each time Settings opens: a session in
// a terminal, or db-clean, changes them, and nothing pushes the change. A
// failed read is not retried on its own; the page offers to read again.
export function useLocalData() {
  return useQuery({
    queryKey: getLocalDataQueryKey(),
    queryFn: ({ signal }) => readLocalData(signal),
    staleTime: 0,
    retry: false,
  })
}

// clean removes what scope reaches and answers what is left. A refusal — a
// file held open, something not the store's own in a file's place, --dry-run
// — throws the API error, whose message is safe to show. Under VITE_MOCK the
// mockup's store forgets what scope reaches, so the read after shows it gone.
async function clean(scope: CleanScope): Promise<LocalData> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockCleanLocalData } = await import('@/dev/mockLocalData.ts')

    return mockCleanLocalData(scope)
  }

  const { data } = await cleanLocalData({ query: { scope }, throwOnError: true })

  return data
}

// useCleanLocalData is the clean, which shows its answer at once and then
// reads the listing again, so what is shown is the directory as it is now.
export function useCleanLocalData(): (scope: CleanScope) => Promise<LocalData> {
  const client = useQueryClient()

  return async (scope) => {
    const left = await clean(scope)
    client.setQueryData(getLocalDataQueryKey(), left)
    await client.invalidateQueries({ queryKey: getLocalDataQueryKey() })

    return left
  }
}
