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

// useStatusChanges reads the status changes the tracker offers an issue, each
// with the fields it needs, for the form that changes it. It reads only once
// the form opens, and again each time it does: what is offered depends on
// where the issue stands now.
export function useStatusChanges(issueKey: string) {
  return useQuery({ ...listStatusChangesOptions({ path: { key: issueKey } }), staleTime: 0 })
}

// IssueWrites are the writes an issue's forms send. Each throws the API
// error on a refusal, whose message is safe to show, and once it is made reads
// the issue again, so its status and assignee show as the tracker now has
// them.
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
  return {
    changeStatus: async (change, fields) => {
      const body = { transition_id: change.id, fields }
      const result = await changeStatus({ path, body, throwOnError: true })

      return readAgain(result.data)
    },
    assign: async (assignee) => {
      const result = await assignIssue({ path, body: { assignee }, throwOnError: true })

      return readAgain(result.data)
    },
    logWork: async (timeSpent, comment) => {
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
