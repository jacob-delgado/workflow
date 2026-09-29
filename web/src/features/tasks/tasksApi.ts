import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  addTask,
  annotateTask,
  completeTask,
  modifyTask,
  startTask,
  stopTask,
  syncTasks,
  trackIssue,
  undoTasks,
} from '@/api/generated'
import { listTasksOptions, listTasksQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { TaskList } from '@/api/generated/types.gen.ts'
import { rememberCompleted, rememberTrack } from './taskMemo.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build, while
// tests can still stub it at runtime.

// useTasks reads your pending tasks, most urgent first, or why no Taskwarrior
// can be asked. Taskwarrior changes outside the page — a task added in a
// terminal, a hook, a sync — and nothing pushes the list, so it is read again
// each time the section opens. A failed read is not retried on its own: the
// server has already said what went wrong, and a Taskwarrior that timed out
// would only be kept waiting again — Retry is the user's to press. Under
// VITE_MOCK it serves the mockup's list, so the section is filled with no
// backend.
export function useTasks() {
  const options = { ...listTasksOptions(), staleTime: 0, retry: false }

  return useQuery(
    import.meta.env.VITE_MOCK === 'true'
      ? {
          ...options,
          queryFn: async (): Promise<TaskList> => {
            const { mockTaskList } = await import('@/dev/mockTasks.ts')

            return mockTaskList()
          },
        }
      : options,
  )
}

// AnsweredTasks is the task list as a read or a write last answered it, and
// when, by the page's clock in milliseconds — 0 while none has.
interface AnsweredTasks {
  list: TaskList | undefined
  answeredAt: number
}

// useAnsweredTasks is the task list the page last read or a write last
// answered, without reading it: the issue's Tasks card lays it over the
// stream's, which a write does not push.
export function useAnsweredTasks(): AnsweredTasks {
  const { data, dataUpdatedAt } = useQuery({ ...listTasksOptions(), enabled: false })

  return { list: data, answeredAt: dataUpdatedAt }
}

// A task write's request: the SDK call, answered with the list after it.
type Send = () => Promise<{ data: TaskList }>

// answerOf sends a write and returns the list it answered. A refusal throws the
// API error, whose message is safe to show. Under VITE_MOCK nothing is sent,
// and the mockup's list answers as it is.
async function answerOf(send: Send): Promise<TaskList> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockTaskList } = await import('@/dev/mockTasks.ts')

    return mockTaskList()
  }

  const { data } = await send()

  return data
}

// TaskWrites are the changes the page makes to your tasks, each answering the
// list after it.
interface TaskWrites {
  add: (line: string) => Promise<TaskList>
  track: (issueKey: string) => Promise<TaskList>
  start: (uuid: string) => Promise<TaskList>
  stop: (uuid: string) => Promise<TaskList>
  complete: (uuid: string) => Promise<TaskList>
  annotate: (uuid: string, text: string) => Promise<TaskList>
  modify: (uuid: string, line: string) => Promise<TaskList>
  undo: () => Promise<TaskList>
  sync: () => Promise<TaskList>
}

// useTaskWrites returns the writes on your tasks. Each answers the whole list
// after it, which replaces the cached one at once, so the section shows what
// Taskwarrior holds now without reading it again. A read still in flight as
// the answer lands may bring the list from before the write, so it is
// canceled first rather than let land after and put that list back. A done
// and a track are remembered too, for what the list cannot say of them until
// the stream catches up: that the task is done, and which task tracks the
// issue where the active context leaves it out.
export function useTaskWrites(): TaskWrites {
  const queryClient = useQueryClient()
  const write = async (send: Send): Promise<TaskList> => {
    const answered = await answerOf(send)
    await queryClient.cancelQueries({ queryKey: listTasksQueryKey() })
    queryClient.setQueryData(listTasksQueryKey(), answered)

    return answered
  }

  return {
    add: (line) => write(() => addTask({ body: { line }, throwOnError: true })),
    track: async (issueKey) => {
      const answered = await write(() =>
        trackIssue({ body: { issue_key: issueKey }, throwOnError: true }),
      )
      if (answered.added !== undefined) {
        rememberTrack(issueKey, answered.added)
      }

      return answered
    },
    start: (uuid) => write(() => startTask({ path: { uuid }, throwOnError: true })),
    stop: (uuid) => write(() => stopTask({ path: { uuid }, throwOnError: true })),
    complete: async (uuid) => {
      const answered = await write(() => completeTask({ path: { uuid }, throwOnError: true }))
      rememberCompleted(uuid)

      return answered
    },
    annotate: (uuid, text) =>
      write(() => annotateTask({ path: { uuid }, body: { text }, throwOnError: true })),
    modify: (uuid, line) =>
      write(() => modifyTask({ path: { uuid }, body: { line }, throwOnError: true })),
    undo: () => write(() => undoTasks({ throwOnError: true })),
    sync: () => write(() => syncTasks({ throwOnError: true })),
  }
}
