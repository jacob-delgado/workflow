import { create } from 'zustand'
import type { Task } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { stillToDo } from './taskWords.ts'

// Remembered is a write on a task the stream has not yet caught up with: the
// task, when the page remembered the write, by its clock in milliseconds, and
// whether a frame received since then has landed.
interface Remembered {
  uuid: string
  rememberedAt: number
  frameSince: boolean
}

// TaskMemo is what this page has just done to your tasks that the stream has
// not yet caught up with, and a task list cannot say: the list holds pending
// tasks alone, so a task done leaves it, and it holds none that Taskwarrior's
// active context hides, as the task a track made can be.
interface TaskMemo {
  // completed are the tasks the page marked done.
  completed: Remembered[]
  // tracks are the tasks the page tracked issues with, by the issue's key.
  tracks: Record<string, Remembered>
}

// The page's memo of its writes, kept outside any one component so it
// outlives the issue's card, which is drawn anew for each issue opened. The
// writes are its only writers, and each frame of the stream what lets go.
export const useTaskMemo = create<TaskMemo>(() => ({ completed: [], tracks: {} }))

// remembered is a write on the task, remembered now.
function remembered(uuid: string): Remembered {
  return { uuid, rememberedAt: Date.now(), frameSince: false }
}

// rememberCompleted notes that the page marked the task done.
export function rememberCompleted(uuid: string): void {
  useTaskMemo.setState((memo) => ({ completed: [...memo.completed, remembered(uuid)] }))
}

// rememberTrack notes that the page tracked the issue with the task.
export function rememberTrack(issueKey: string, uuid: string): void {
  useTaskMemo.setState((memo) => ({ tracks: { ...memo.tracks, [issueKey]: remembered(uuid) } }))
}

// aged is the write once a frame received at receivedAt has landed, or
// undefined once it lets go: frames on one stream are read in turn, so only
// the first to land after a write can have been read before it, and the one
// after that shows the task as Taskwarrior holds it, whatever has changed it
// since the write.
function aged(entry: Remembered, receivedAt: number): Remembered | undefined {
  if (entry.frameSince) {
    return undefined
  }

  return receivedAt > entry.rememberedAt ? { ...entry, frameSince: true } : entry
}

// caughtUp is the memo once a frame received at receivedAt holds the linked
// tasks: it lets go of a task done once a frame has it done or no longer holds
// it, and of a track once a frame holds its task — and of either once the
// second frame received since has landed, since a done or a track undone
// before the stream read Taskwarrior again is never seen at all. A frame read
// before the write can land after its answer, so the first that still has the
// task as it was lets go of nothing.
function caughtUp(memo: TaskMemo, linked: Task[], receivedAt: number): TaskMemo {
  const held = new Map(linked.map((task) => [task.uuid, task]))
  const stillPending = (entry: Remembered) => {
    const task = held.get(entry.uuid)

    return task !== undefined && stillToDo(task)
  }
  const kept = (entry: Remembered) => aged(entry, receivedAt) ?? []

  return {
    completed: memo.completed.filter(stillPending).flatMap(kept),
    tracks: Object.fromEntries(
      Object.entries(memo.tracks)
        .filter(([, entry]) => !held.has(entry.uuid))
        .flatMap(([issueKey, entry]) => {
          const next = aged(entry, receivedAt)

          return next === undefined ? [] : [[issueKey, next] as const]
        }),
    ),
  }
}

// Every frame, whichever section is open, since one that catches up while the
// card is not drawn must still let go of what it caught up with.
useSnapshotStore.subscribe((state, previous) => {
  const frame = state.snapshot
  if (frame !== null && frame !== previous.snapshot) {
    useTaskMemo.setState((memo) => caughtUp(memo, frame.tasks.linked, state.receivedAt))
  }
})
