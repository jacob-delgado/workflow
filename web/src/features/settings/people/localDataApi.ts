import { useQuery, useQueryClient } from '@tanstack/react-query'
import { removeLocalData, getLocalData } from '@/api/generated'
import {
  getLocalDataQueryKey,
  getPeopleQueryKey,
  getRepoGroupsQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type { LocalData } from '@/api/generated/types.gen.ts'

// RemoveScope is how much a removal takes: the cache alone, or the kept
// associations too.
export type RemoveScope = 'cache' | 'all'

// readLocalData reads the store's directory and its database files.
async function readLocalData(signal?: AbortSignal): Promise<LocalData> {
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

// remove removes what scope reaches and answers what is left. A refusal — a
// file held open, something not the store's own in a file's place, --dry-run
// — throws the API error, whose message is safe to show.
async function remove(scope: RemoveScope): Promise<LocalData> {
  const { data } = await removeLocalData({ query: { scope }, throwOnError: true })

  return data
}

// useRemoveLocalData is the removal, which shows its answer at once and then
// reads the listing again, so what is shown is the directory as it is now —
// after a refused removal too, which may have removed some files before one
// would not go. Removing everything takes the people and groups with the
// kept file, so they are read again, and Settings never saves the old ones
// back.
export function useRemoveLocalData(): (scope: RemoveScope) => Promise<LocalData> {
  const client = useQueryClient()

  return async (scope) => {
    try {
      const left = await remove(scope)
      client.setQueryData(getLocalDataQueryKey(), left)

      return left
    } finally {
      await Promise.all([
        client.invalidateQueries({ queryKey: getLocalDataQueryKey() }),
        ...(scope === 'all'
          ? [getPeopleQueryKey(), getRepoGroupsQueryKey()].map((queryKey) =>
              client.invalidateQueries({ queryKey }),
            )
          : []),
      ])
    }
  }
}
