import { useQuery, useQueryClient } from '@tanstack/react-query'
import { forgetPerson, getPeople, getRepoGroups, setRepoGroups } from '@/api/generated'
import {
  getPeopleQueryKey,
  getRepoGroupsQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type { People, PersonLink, RepoGroups } from '@/api/generated/types.gen.ts'
import { savePerson } from '@/features/messaging/slackApi.ts'

// readPeople reads every decided owner, then the branch's undecided ones.
async function readPeople(signal?: AbortSignal): Promise<People> {
  const { data } = await getPeople({ signal, throwOnError: true })

  return data
}

// usePeople reads the owners each time Settings opens: an announcement in a
// terminal links owners too, and nothing pushes the change. A failed read is
// not retried on its own; the page offers to read again.
export function usePeople() {
  return useQuery({
    queryKey: getPeopleQueryKey(),
    queryFn: ({ signal }) => readPeople(signal),
    staleTime: 0,
    retry: false,
  })
}

// forget drops what was decided for owner, and answers the owners after it.
async function forget(owner: string): Promise<People> {
  const { data } = await forgetPerson({ query: { owner }, throwOnError: true })

  return data
}

// usePeopleWrites are a link and a forget, each showing the owners it
// answers at once. A refusal throws the API error, whose message is safe to
// show.
export function usePeopleWrites() {
  const client = useQueryClient()
  const shown = (people: People) => {
    client.setQueryData(getPeopleQueryKey(), people)

    return people
  }

  return {
    link: async (link: PersonLink) => shown(await savePerson(link)),
    forget: async (owner: string) => shown(await forget(owner)),
  }
}

// readRepoGroups reads the groups this repository's announcements may tag.
async function readRepoGroups(signal?: AbortSignal): Promise<RepoGroups> {
  const { data } = await getRepoGroups({ signal, throwOnError: true })

  return data
}

// useRepoGroups reads the repository's groups each time Settings opens.
export function useRepoGroups() {
  return useQuery({
    queryKey: getRepoGroupsQueryKey(),
    queryFn: ({ signal }) => readRepoGroups(signal),
    staleTime: 0,
    retry: false,
  })
}

// saveGroups keeps the groups ids name as the repository's, each labeled as
// Slack's directory has it.
async function saveGroups(ids: string[]): Promise<RepoGroups> {
  const { data } = await setRepoGroups({ body: { ids }, throwOnError: true })

  return data
}

// useSaveRepoGroups is the save, which shows the groups it answers at once.
export function useSaveRepoGroups(): (ids: string[]) => Promise<RepoGroups> {
  const client = useQueryClient()

  return async (ids) => {
    const saved = await saveGroups(ids)
    client.setQueryData(getRepoGroupsQueryKey(), saved)

    return saved
  }
}
