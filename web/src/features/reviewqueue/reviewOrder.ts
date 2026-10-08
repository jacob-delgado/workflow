import type { ReviewRequest } from '@/api/generated/types.gen.ts'

// ReviewOrder is the order the queue is listed in, as the terminal's s and
// `workflow reviews --sort` choose it.
export type ReviewOrder = 'oldest' | 'newest' | 'repository'

// orderWords names each order, in the order they are offered.
export const orderWords: Record<ReviewOrder, string> = {
  oldest: 'Oldest first',
  newest: 'Newest first',
  repository: 'By repository',
}

// RepositoryGroup is a repository's requests, by the repository as the forge
// named it: empty for none.
export interface RepositoryGroup {
  repository: string
  requests: ReviewRequest[]
}

// repositoryHeading heads a repository's requests: by its name, or as having
// none when the forge named none.
export function repositoryHeading(repository: string): string {
  return repository === '' ? 'No repository' : repository
}

// ordered is the queue in order, as forge.OldestFirst, NewestFirst and
// ByRepository have it: requests opened at the same moment keep the server's
// order, and by repository is by name, the longest-waiting first within each.
export function ordered(requests: ReviewRequest[], order: ReviewOrder): ReviewRequest[] {
  const oldest = [...requests].sort((left, right) => openedAt(left) - openedAt(right))

  switch (order) {
    case 'oldest':
      return oldest
    case 'newest':
      return [...requests].sort((left, right) => openedAt(right) - openedAt(left))
    case 'repository':
      return oldest.sort((left, right) => byteOrder(left.repository, right.repository))
  }
}

// byRepository is the requests, already in order, cut into a group per
// repository in the order they first appear.
export function byRepository(requests: ReviewRequest[]): RepositoryGroup[] {
  const groups: RepositoryGroup[] = []

  for (const request of requests) {
    const last = groups.at(-1)
    if (last?.repository === request.repository) {
      last.requests.push(request)
    } else {
      groups.push({ repository: request.repository, requests: [request] })
    }
  }

  return groups
}

// openedAt is when request was opened, by the clock; one the forge gave no
// time for counts as the earliest, as the server's own order has it.
function openedAt(request: ReviewRequest): number {
  return request.opened_at === undefined ? Number.MIN_SAFE_INTEGER : Date.parse(request.opened_at)
}

// byteOrder compares code unit by code unit, as Go's strings.Compare does
// for the ASCII a repository's name is spelled in, rather than by locale, so
// both surfaces list the repositories alike.
function byteOrder(left: string, right: string): number {
  if (left === right) {
    return 0
  }

  return left < right ? -1 : 1
}
