import { queryOptions, useQuery, useQueryClient } from '@tanstack/react-query'
import { getConfig, updateConfig } from '@/api/generated'
import { getConfigQueryKey, listViewsQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { Config } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build, while
// tests can still stub it at runtime.

// ConfigRead is the configuration as a read or a save returned it, with the
// revision of the file it stands for: what the next save names, so it is
// written only over the file as it was read. Undefined when the answer named
// none, as under VITE_MOCK, where no file is written.
export interface ConfigRead {
  config: Config
  revision: string | undefined
}

// revisionOf is the revision an answer names: its ETag, as the server wrote it.
function revisionOf(response: Response): string | undefined {
  return response.headers.get('ETag') ?? undefined
}

// readConfig reads the configuration with secrets masked, and its revision.
// Under VITE_MOCK it serves the fixture (code-split, dev-only) so Settings
// works with no backend.
async function readConfig(signal?: AbortSignal): Promise<ConfigRead> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockConfig } = await import('@/dev/mockConfig.ts')

    return { config: mockConfig, revision: undefined }
  }

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
// reason behind "Loading…" for seconds. Retry is the user's to press. Under
// VITE_MOCK there is no file to change, and reading the fixture again would
// undo a save's echo.
function configQuery() {
  return queryOptions({
    queryKey: getConfigQueryKey(),
    queryFn: async ({ signal, client, queryKey }) => {
      const before = client.getQueryData<ConfigRead>(queryKey)
      const read = await readConfig(signal)
      // A read takes up an edit made on disk, which can change the issue views,
      // so one at a revision other than the cached read's (or with none cached,
      // where it cannot tell) reads their list again, as a save and Reload do.
      if (before?.revision !== read.revision) {
        void client.invalidateQueries({ queryKey: listViewsQueryKey() })
      }

      return read
    },
    staleTime: import.meta.env.VITE_MOCK === 'true' ? Infinity : 0,
    refetchOnReconnect: false,
    retry: false,
  })
}

// useConfigRead reads the configuration with the revision a save names.
export function useConfigRead() {
  return useQuery(configQuery())
}

// useConfig reads the configuration alone, for a surface that only reads it.
export function useConfig() {
  return useQuery({ ...configQuery(), select: (read) => read.config })
}

// saveConfig writes the whole configuration back over the revision named, and
// returns it as stored, with secrets re-masked, and the revision written. A
// secret left at its masked value keeps the stored one — the server preserves
// it — so the form need not special-case them. Under VITE_MOCK the save is a
// no-op that echoes the input.
async function saveConfig(config: Config, over: string | undefined): Promise<ConfigRead> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return { config, revision: over }
  }

  const { data, response } = await updateConfig({
    body: config,
    headers: over === undefined ? undefined : { 'If-Match': over },
    throwOnError: true,
  })

  return { config: data, revision: revisionOf(response) }
}

// useSaveConfig saves the configuration over the revision it was read at, and
// refreshes the cached copy with the stored result, so a surface that reads the
// cache before its next read — and every one, under VITE_MOCK, which never
// reads again — shows the save. The issue views live in the configuration too,
// so their list is read again: the view select offers only the views the
// server lists.
export function useSaveConfig(): (config: Config, over: string | undefined) => Promise<ConfigRead> {
  const queryClient = useQueryClient()

  return async (config, over) => {
    const saved = await saveConfig(config, over)
    queryClient.setQueryData(configQuery().queryKey, saved)
    void queryClient.invalidateQueries({ queryKey: listViewsQueryKey() })

    return saved
  }
}

// useReloadConfig reads the configuration again and returns it: what Settings
// offers when a save finds the file has changed. The read goes around the
// query, so one that fails leaves the query, and the form it seeded, as they
// were; one that answers replaces the cached copy every surface shares. A
// reload can take up an edit to the views, so their list is read again, as
// after a save.
export function useReloadConfig(): () => Promise<ConfigRead> {
  const queryClient = useQueryClient()

  return async () => {
    const read = await readConfig()
    queryClient.setQueryData(configQuery().queryKey, read)
    void queryClient.invalidateQueries({ queryKey: listViewsQueryKey() })

    return read
  }
}

// changedSinceRead reports a save refused because the file has moved on from
// the revision it named — edited on disk, or saved from another tab — which
// reading it again, not saving again, answers.
export function changedSinceRead(caught: unknown): boolean {
  return (
    typeof caught === 'object' && caught !== null && 'code' in caught && caught.code === 'conflict'
  )
}
