import { useQuery, useQueryClient } from '@tanstack/react-query'
import { assignIssue, changeStatus, logWork } from '@/api/generated'
import {
  getIssueQueryKey,
  listStatusChangesOptions,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type {
  AssignedIssue,
  FieldEntry,
  LoggedWork,
  MovedIssue,
  StatusChange,
} from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build, while
// tests can still stub it at runtime.

// useStatusChanges reads the status changes the tracker offers an issue, each
// with the fields it needs, for the form that changes it. It reads only once
// the form opens, and again each time it does: what is offered depends on
// where the issue stands now. Under VITE_MOCK it serves the fixture's changes.
export function useStatusChanges(issueKey: string) {
  const options = { ...listStatusChangesOptions({ path: { key: issueKey } }), staleTime: 0 }

  return useQuery(
    import.meta.env.VITE_MOCK === 'true'
      ? {
          ...options,
          queryFn: async (): Promise<StatusChange[]> => {
            const { mockStatusChanges } = await import('@/dev/mockIssues.ts')

            return mockStatusChanges
          },
        }
      : options,
  )
}

// IssueWrites are the writes an issue's forms send. Each throws the API
// error on a refusal, whose message is safe to show, and once it is made reads
// the issue again, so its status and assignee show as the tracker now has
// them. Under VITE_MOCK each answers as the tracker would, sending nothing.
interface IssueWrites {
  changeStatus: (change: StatusChange, fields: FieldEntry[]) => Promise<MovedIssue>
  assign: (assignee: string) => Promise<AssignedIssue>
  logWork: (timeSpent: string, comment: string) => Promise<LoggedWork>
}

export function useIssueWrites(issueKey: string): IssueWrites {
  const queryClient = useQueryClient()
  const path = { key: issueKey }
  const readAgain = async <T>(answer: T): Promise<T> => {
    await queryClient.invalidateQueries({ queryKey: getIssueQueryKey({ path }) })

    return answer
  }
  const mock = import.meta.env.VITE_MOCK === 'true'

  return {
    changeStatus: async (change, fields) => {
      if (mock) {
        return { key: issueKey, status: change.to_status }
      }

      const body = { transition_id: change.id, fields }
      const result = await changeStatus({ path, body, throwOnError: true })

      return readAgain(result.data)
    },
    assign: async (assignee) => {
      if (mock) {
        return { key: issueKey, assignee: assignee.trim() }
      }

      const result = await assignIssue({ path, body: { assignee }, throwOnError: true })

      return readAgain(result.data)
    },
    logWork: async (timeSpent, comment) => {
      if (mock) {
        return { key: issueKey, time_spent: timeSpent.trim() }
      }

      const note = comment.trim() === '' ? {} : { comment }
      const result = await logWork({
        path,
        body: { time_spent: timeSpent, ...note },
        throwOnError: true,
      })

      return readAgain(result.data)
    },
  }
}
