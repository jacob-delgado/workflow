import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  addFavorite,
  getDirectories,
  getRepositories,
  removeFavorite,
  switchRepository,
} from '@/api/generated'
import { getRepositoriesQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { DirectoryListing, Repositories } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'

// readRepositories reads where the server works and your favorites.
async function readRepositories(signal?: AbortSignal): Promise<Repositories> {
  const { data } = await getRepositories({ signal, throwOnError: true })

  return data
}

// useRepositories reads where the server works and your favorites each time
// the section opens: a favorite is a directory on disk, which nothing pushes.
export function useRepositories() {
  return useQuery({
    queryKey: getRepositoriesQueryKey(),
    queryFn: ({ signal }) => readRepositories(signal),
    staleTime: 0,
    retry: false,
  })
}

// readDirectories lists the directories in path, or where the server works
// when path is empty.
async function readDirectories(path: string, signal?: AbortSignal): Promise<DirectoryListing> {
  const { data } = await getDirectories({
    query: path === '' ? {} : { path },
    signal,
    throwOnError: true,
  })

  return data
}

// useDirectories lists the directories in path, kept while it is browsed.
export function useDirectories(path: string) {
  return useQuery({
    queryKey: ['directories', path],
    queryFn: ({ signal }) => readDirectories(path, signal),
    retry: false,
  })
}

// useSwitchTo switches the directory the server works in, and tells the page
// at once where it works now, rather than when the stream reconnects: until
// then its writes would name the directory left, which the server refuses.
// useFollowSwitch, hearing the change, reads every section again; before the
// stream's first frame there is no page state to follow, so they are read
// again here.
export function useSwitchTo(): (dir: string) => Promise<Repositories> {
  const client = useQueryClient()

  return async (dir) => {
    const { data } = await switchRepository({ body: { dir }, throwOnError: true })
    const snapshot = useSnapshotStore.getState().snapshot
    if (snapshot === null) {
      await client.resetQueries()
    } else {
      useSnapshotStore.setState({ snapshot: { ...snapshot, here: data.here.dir } })
    }

    return data
  }
}

// useFavorite marks a directory a favorite, or forgets it, and keeps what the
// server answers as the section's.
export function useFavorite(): (dir: string, favorite: boolean) => Promise<Repositories> {
  const client = useQueryClient()

  return async (dir, favorite) => {
    const { data } = favorite
      ? await addFavorite({ body: { dir }, throwOnError: true })
      : await removeFavorite({ query: { dir }, throwOnError: true })
    client.setQueryData(getRepositoriesQueryKey(), data)

    return data
  }
}
