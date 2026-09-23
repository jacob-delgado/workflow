import { useQuery, useQueryClient } from '@tanstack/react-query'
import { updateConfig } from '@/api/generated'
import { getConfigOptions, listViewsQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { Config } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build, while
// tests can still stub it at runtime.

// useConfig reads the configuration with secrets masked. Under VITE_MOCK it
// serves the fixture (code-split, dev-only) so Settings works with no backend.
export function useConfig() {
  const options = getConfigOptions()

  return useQuery(
    import.meta.env.VITE_MOCK === 'true'
      ? {
          ...options,
          queryFn: async (): Promise<Config> => {
            const { mockConfig } = await import('@/dev/mockConfig.ts')

            return mockConfig
          },
        }
      : options,
  )
}

// saveConfig writes the whole configuration back and returns it as stored, with
// secrets re-masked. A secret left at its masked value keeps the stored one —
// the server preserves it — so the form need not special-case them. Under
// VITE_MOCK the save is a no-op that echoes the input.
async function saveConfig(config: Config): Promise<Config> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return config
  }

  const result = await updateConfig({ body: config, throwOnError: true })

  return result.data
}

// useSaveConfig saves the configuration and refreshes the cached copy with the
// stored result. The refresh matters: the config query never refetches on its
// own (staleTime is Infinity), so without it a reopened Settings would show the
// pre-save values. The issue views live in the configuration too, so their list
// is read again: the view select offers only the views the server lists.
export function useSaveConfig(): (config: Config) => Promise<Config> {
  const queryClient = useQueryClient()

  return async (config) => {
    const saved = await saveConfig(config)
    queryClient.setQueryData(getConfigOptions().queryKey, saved)
    void queryClient.invalidateQueries({ queryKey: listViewsQueryKey() })

    return saved
  }
}
