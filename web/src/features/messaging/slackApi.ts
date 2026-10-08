import { useQuery } from '@tanstack/react-query'
import { getSlackGroups, getSlackMembers, linkPerson } from '@/api/generated'
import {
  getSlackGroupsQueryKey,
  getSlackMembersQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type { People, PersonLink, SlackDirectory } from '@/api/generated/types.gen.ts'

// directoryHold is how long a directory read is kept before it is asked
// again: the server keeps its own for ten minutes, so asking sooner reads the
// same.
const directoryHold = 10 * 60 * 1000

// readMembers reads the people in channel — the configured one when empty.
async function readMembers(channel: string, signal?: AbortSignal): Promise<SlackDirectory> {
  const query = channel === '' ? {} : { channel }
  const { data } = await getSlackMembers({ query, signal, throwOnError: true })

  return data
}

// useSlackMembers is the people in channel an owner can be linked to, read
// only while enabled — while some owner waits to be linked. A token without
// the scope the read needs answers none, naming the scope.
export function useSlackMembers(channel: string, enabled: boolean) {
  return useQuery({
    queryKey: getSlackMembersQueryKey(channel === '' ? {} : { query: { channel } }),
    queryFn: ({ signal }) => readMembers(channel, signal),
    enabled,
    staleTime: directoryHold,
    retry: false,
  })
}

// readGroups reads the workspace's user groups.
async function readGroups(signal?: AbortSignal): Promise<SlackDirectory> {
  const { data } = await getSlackGroups({ signal, throwOnError: true })

  return data
}

// useSlackGroups is the workspace's user groups, a team owner's choices and
// the repository's, read only while enabled.
export function useSlackGroups(enabled: boolean) {
  return useQuery({
    queryKey: getSlackGroupsQueryKey(),
    queryFn: ({ signal }) => readGroups(signal),
    enabled,
    staleTime: directoryHold,
    retry: false,
  })
}

// savePerson keeps whom an owner is on Slack, or that they are not on it, and
// answers every owner as they stand after it. The label kept is the one
// Slack's directory gives, never the page's. A refusal throws the API error,
// whose message is safe to show.
export async function savePerson(link: PersonLink): Promise<People> {
  const { data } = await linkPerson({ body: link, throwOnError: true })

  return data
}
