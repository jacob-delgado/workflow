import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getSetup, setUp } from '@/api/generated'
import {
  getConfigQueryKey,
  getSetupQueryKey,
  listViewsQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type { SetupRequest, SetupResult } from '@/api/generated/types.gen.ts'

// useSetupOffer reads where a first configuration file may go, and whether the
// keychain can keep the token, each time Settings offers a setup. A failed
// read is not retried on its own; the page offers to read again.
export function useSetupOffer() {
  return useQuery({
    queryKey: getSetupQueryKey(),
    queryFn: async ({ signal }) => (await getSetup({ signal, throwOnError: true })).data,
    staleTime: 0,
    retry: false,
  })
}

// useSetUp writes the first configuration file and answers what it wrote. The
// server works with the file from then on, so the configuration, and the
// issue views it names, are read again; the event stream reconnects on its
// own, the server having ended it. A refusal — a check that did not pass, a
// file already there, --dry-run — throws the API error, whose message is safe
// to show.
export function useSetUp(): (request: SetupRequest) => Promise<SetupResult> {
  const client = useQueryClient()

  return async (request) => {
    const { data } = await setUp({ body: request, throwOnError: true })
    await client.invalidateQueries({ queryKey: getConfigQueryKey() })
    void client.invalidateQueries({ queryKey: listViewsQueryKey() })

    return data
  }
}
