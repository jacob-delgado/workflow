import { type QueryClient, queryOptions, useQuery, useQueryClient } from '@tanstack/react-query'
import { getConfig, updateConfig } from '@/api/generated'
import {
  getConfigQueryKey,
  getKeysQueryKey,
  listViewsQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type { Config } from '@/api/generated/types.gen.ts'

// ConfigRead is the configuration as a read or a save returned it, with the
// revision of the file it stands for: what the next save names, so it is
// written only over the file as it was read. Undefined when the answer named
// none.
export interface ConfigRead {
  config: Config
  revision: string | undefined
}

// revisionOf is the revision an answer names: its ETag, as the server wrote it.
function revisionOf(response: Response): string | undefined {
  return response.headers.get('ETag') ?? undefined
}

// readConfig reads the configuration with secrets masked, and its revision.
async function readConfig(signal?: AbortSignal): Promise<ConfigRead> {
  const { data, response } = await getConfig({ signal, throwOnError: true })

  return { config: data, revision: revisionOf(response) }
}

// configQuery is the one read of the configuration every surface shares. It is
// the one query that reads again whenever a surface showing it opens: the file
// can change on disk, which no event reports, and a form seeded from an older
// read only learns so when its save is refused. It reads again at no other
// time — a reconnect says nothing about the file, and a read that failed then
// would take an open form, and the edits in it, away — and a failed read is
// not retried on its own: a problem answer, such as a file on disk that is not
// valid, stands until the file changes, and retrying would only hide its
// reason behind "Reading…" for seconds. Try again is the user's to press.
function configQuery() {
  return queryOptions({
    queryKey: getConfigQueryKey(),
    queryFn: async ({ signal, client, queryKey }) => {
      const before = client.getQueryData<ConfigRead>(queryKey)
      const read = await readConfig(signal)
      // A read takes up an edit made on disk, which can change the issue views
      // and the keys, so one at a revision other than the cached read's (or
      // with none cached, where it cannot tell) reads them again, as a save and
      // Reload do.
      if (before?.revision !== read.revision) {
        readAgainAfter(client)
      }

      return read
    },
    staleTime: 0,
    refetchOnReconnect: false,
    retry: false,
  })
}

// useConfigRead reads the configuration with the revision a save names.
export function useConfigRead() {
  return useQuery(configQuery())
}

// saveConfig writes the whole configuration back over the revision named, and
// returns it as stored, with secrets re-masked, and the revision written. A
// secret left at its masked value keeps the stored one — the server preserves
// it — so the form need not special-case them.
async function saveConfig(config: Config, over: string | undefined): Promise<ConfigRead> {
  const { data, response } = await updateConfig({
    body: config,
    headers: over === undefined ? undefined : { 'If-Match': over },
    throwOnError: true,
  })

  return { config: data, revision: revisionOf(response) }
}

// useSaveConfig saves the configuration over the revision it was read at, and
// refreshes the cached copy with the stored result, so a surface that reads the
// cache before its next read shows the save. The issue views and the keys live in the
// configuration too, so they are read again: the view select offers only the
// views the server lists, and a key moved or turned on works at once.
export function useSaveConfig(): (config: Config, over: string | undefined) => Promise<ConfigRead> {
  const queryClient = useQueryClient()

  return async (config, over) => {
    const saved = await saveConfig(config, over)
    queryClient.setQueryData(configQuery().queryKey, saved)
    readAgainAfter(queryClient)

    return saved
  }
}

// useReloadConfig reads the configuration again and returns it: what Settings
// offers when a save finds the file has changed. The read goes around the
// query, so one that fails leaves the query, and the form it seeded, as they
// were; one that answers replaces the cached copy every surface shares. A
// reload can take up an edit to the views or the keys, so they are read again,
// as after a save.
export function useReloadConfig(): () => Promise<ConfigRead> {
  const queryClient = useQueryClient()

  return async () => {
    const read = await readConfig()
    queryClient.setQueryData(configQuery().queryKey, read)
    readAgainAfter(queryClient)

    return read
  }
}

// readAgainAfter reads again what the configuration decides beyond the form:
// the issue views, which the view select offers, and the keys, which ui.keys
// moves and ui.web_shortcuts turns on.
function readAgainAfter(client: QueryClient): void {
  void client.invalidateQueries({ queryKey: listViewsQueryKey() })
  void client.invalidateQueries({ queryKey: getKeysQueryKey() })
}

// changedSinceRead reports a save refused because the file has moved on from
// the revision it named — edited on disk, or saved from another tab — which
// reading it again, not saving again, answers.
export function changedSinceRead(caught: unknown): boolean {
  return (
    typeof caught === 'object' && caught !== null && 'code' in caught && caught.code === 'conflict'
  )
}
