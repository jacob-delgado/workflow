import { useEffect, useState } from 'react'
import type { Task } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { StateMark } from '@/shell/StateMark.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { elapsedWords, isActive } from './taskWords.ts'

// How often the clock the task words count from moves on: they count minutes.
const tick = 60_000

// useNow is the time the task words count from — how long a task has run, how
// soon one is due — read as the component mounts and again each minute, so a
// page left open keeps counting.
export function useNow(): number {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const timer = setInterval(() => {
      setNow(Date.now())
    }, tick)

    return () => {
      clearInterval(timer)
    }
  }, [])

  return now
}

// ActiveTask is the header's chip for the task you have started: its mark, what
// it is and how long it has run, as the terminal's spine ends with it — a
// button that opens the Tasks section. Nothing is drawn while no task is
// started, nor where Taskwarrior is not available. The chip sits in the room
// between the header's ends, which it never grows past: from a flex basis of
// nothing, the header's row never wraps for it, and a long description is cut
// short rather than push the header's controls onto a row of their own. The
// chip is keyed on the task, so its clock starts as the task appears, not as
// the page opened.
export function ActiveTask() {
  const tasks = useSnapshotStore((state) => state.snapshot?.tasks)
  const active = tasks?.available === true ? tasks.active : undefined

  if (active === undefined || !isActive(active)) {
    return null
  }

  return (
    <div className="flex min-w-0 flex-1 basis-0 justify-center">
      <Chip key={active.uuid} task={active} />
    </div>
  )
}

// Chip is the started task, as the header draws it.
function Chip({ task }: { task: Task & { start: string } }) {
  const setSection = useUiStore((state) => state.setSection)
  const now = useNow()

  // Not a Button: the header's chip, drawn as the task it names, not as an act.
  return (
    <button
      type="button"
      title={task.description}
      onClick={() => {
        setSection('tasks')
      }}
      className="relative flex max-w-md min-w-0 items-center gap-2 rounded-md border border-border px-2.5 py-1 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      <StateMark state="in-flight" className="text-taskwarrior" />
      <span className="sr-only">Active task: </span>
      <span className="truncate">{task.description}</span>
      <span className="shrink-0 text-muted-foreground tabular-nums">
        {elapsedWords(task.start, now)}
      </span>
    </button>
  )
}
