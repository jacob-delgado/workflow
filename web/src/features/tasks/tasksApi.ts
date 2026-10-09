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

// useTasks reads your pending tasks, most urgent first, or why no Taskwarrior
// can be asked. Taskwarrior changes outside the page — a task added in a
// terminal, a hook, a sync — and nothing pushes the list, so it is read again
// each time the section opens. A wait passing changes where a task stands
// with no write at all, so the list is read again, too, once the earliest wait
// still ahead has passed. A failed read is not retried on its own: the server
// has already said what went wrong, and a Taskwarrior that timed out would
// only be kept waiting again — Try again is the user's to press.
export function useTasks() {
  return useQuery({
    ...listTasksOptions(),
    staleTime: 0,
    retry: false,
    refetchInterval: (query) => untilNextWait(query.state.data, Date.now()),
  })
}

// waitPassed is how long past a wait the list is read again, so the server's
// clock is past it too.
const waitPassed = 1_000

// longestTimer is the longest a browser timer can wait: one longer fires at
// once.
const longestTimer = 2_147_483_647

// untilNextWait is how long from now until the earliest wait still ahead among
// the listed tasks has passed, or false when none is ahead.
function untilNextWait(list: TaskList | undefined, now: number): number | false {
  const ahead = (list?.tasks ?? [])
    .map((task) => (task.wait === undefined ? Number.NaN : Date.parse(task.wait) - now))
    .filter((left) => left > 0)
  if (ahead.length === 0) {
    return false
  }

  return Math.min(Math.min(...ahead) + waitPassed, longestTimer)
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

// A task write's request: the SDK call, answered with the list after it. A
// refusal throws the API error, whose message is safe to show.
type Send = () => Promise<{ data: TaskList }>

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
// canceled first rather than let land after and put that list back. A write
// that landed but whose list the server could not read again answers it
// unavailable, saying why; the list shown stays, and is read once more,
// rather than give way to a Taskwarrior that is there. A done and a track are
// remembered too, for what the list cannot say of them until the stream
// catches up: the task done, as the done's answer describes it, and which task
// tracks the issue where the active context leaves it out.
export function useTaskWrites(): TaskWrites {
  const queryClient = useQueryClient()
  const write = async (send: Send): Promise<TaskList> => {
    const { data: answered } = await send()
    await queryClient.cancelQueries({ queryKey: listTasksQueryKey() })
    if (answered.available) {
      queryClient.setQueryData(listTasksQueryKey(), answered)
    } else {
      void queryClient.invalidateQueries({ queryKey: listTasksQueryKey() })
    }

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
      if (answered.done !== undefined) {
        rememberCompleted(answered.done)
      }

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
