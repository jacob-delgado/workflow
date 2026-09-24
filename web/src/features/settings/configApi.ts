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
async function readConfig(signal: AbortSignal): Promise<ConfigRead> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockConfig } = await import('@/dev/mockConfig.ts')

    return { config: mockConfig, revision: undefined }
  }

  const { data, response } = await getConfig({ signal, throwOnError: true })

  return { config: data, revision: revisionOf(response) }
}

// configQuery is the one read of the configuration every surface shares.
function configQuery() {
  return queryOptions({
    queryKey: getConfigQueryKey(),
    queryFn: ({ signal }) => readConfig(signal),
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
// refreshes the cached copy with the stored result. The refresh matters: the
// config query never refetches on its own (staleTime is Infinity), so without
// it a reopened Settings would show the pre-save values. The issue views live
// in the configuration too, so their list is read again: the view select
// offers only the views the server lists.
export function useSaveConfig(): (config: Config, over: string | undefined) => Promise<ConfigRead> {
  const queryClient = useQueryClient()

  return async (config, over) => {
    const saved = await saveConfig(config, over)
    queryClient.setQueryData(configQuery().queryKey, saved)
    void queryClient.invalidateQueries({ queryKey: listViewsQueryKey() })

    return saved
  }
}
