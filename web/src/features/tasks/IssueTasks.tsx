import type { Task, TaskList } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { Meta } from '@/lib/Meta.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { capitalized } from '@/lib/utils.ts'
import { StateMark } from '@/shell/StateMark.tsx'
import { useNow } from './ActiveTask.tsx'
import { TaskVerbs } from './TaskDetail.tsx'
import { useTaskMemo } from './taskMemo.ts'
import { useAnsweredTasks, useTaskWrites } from './tasksApi.ts'
import {
  addedName,
  dueWords,
  elapsedWords,
  isActive,
  linkedTo,
  markOf,
  statusWords,
  stillToDo,
  taskName,
  taskNumber,
} from './taskWords.ts'

// IssueTasks is the Tasks card of an issue's detail: each task linked to the
// issue, with its mark and how it stands, and — for one still to do — the
// buttons that start or stop it and mark it done; and, while no linked task is
// still to do and the page has not just tracked the issue, the button that
// tracks it in Taskwarrior. It reads the stream, with the task list a write
// last answered laid over it and what the page has just done, so a write shows
// at once rather than a frame later. Nothing is drawn until the stream says
// Taskwarrior has answered, as the terminal draws no Tasks block until it has.
export function IssueTasks({ issueKey }: { issueKey: string }) {
  const tasks = useSnapshotStore((state) => state.snapshot?.tasks)
  const streamedAt = useSnapshotStore((state) => state.receivedAt)
  const answered = useAnsweredTasks()
  const completed = useTaskMemo((memo) => memo.completed)
  const trackedByPage = useTaskMemo((memo) => memo.tracks[issueKey] !== undefined)
  const now = useNow()
  const outcome = useOutcome()

  if (tasks?.available !== true) {
    return null
  }

  const listed = linkedTo(answered.list?.tasks ?? [], issueKey)
  const newest = newestOf(
    linkedTo(tasks.linked, issueKey),
    listed,
    answered.answeredAt >= streamedAt,
  )
  const linked = withDone(newest, new Set(completed.map((write) => write.uuid)), listed)
  const tracked = trackedByPage || linked.some(stillToDo)

  return (
    <section aria-labelledby="issue-tasks-heading" className="flex flex-col gap-group">
      <h3 id="issue-tasks-heading" className="text-base font-semibold">
        Tasks
      </h3>
      {linked.length === 0 ? null : (
        <ul aria-labelledby="issue-tasks-heading" className="flex flex-col gap-group">
          {linked.map((task) => (
            <LinkedTask key={task.uuid} task={task} now={now} teller={outcome} />
          ))}
        </ul>
      )}
      {tracked ? null : (
        <TrackIssue issueKey={issueKey} unlinked={linked.length === 0} teller={outcome} />
      )}
      <OutcomeLine said={outcome.said} />
    </section>
  )
}

// newestOf is the issue's tasks from whichever of the stream and the list a
// read or a write last answered is newer, the list on a tie. A list as new lays
// its version of each task over the stream's, by uuid, and adds any only it
// holds — the one a write has just made; the stream's frame holds every task
// linked to the issue, whatever the active context, so a newer one stands
// alone.
function newestOf(streamed: Task[], listed: Task[], listNewer: boolean): Task[] {
  if (!listNewer) {
    return streamed
  }

  const byUUID = new Map(streamed.map((task) => [task.uuid, task]))
  for (const task of listed) {
    byUUID.set(task.uuid, task)
  }

  return [...byUUID.values()]
}

// withDone shows done each task the page has marked done that the stream has
// not caught up with, which the list cannot say, since a task done leaves it —
// unless the list holds it: a list holding it was answered after the done, as
// an undo that brought it back answers.
function withDone(tasks: Task[], completed: Set<string>, listed: Task[]): Task[] {
  const stillListed = new Set(listed.map((task) => task.uuid))

  return tasks.map((task) =>
    completed.has(task.uuid) && !stillListed.has(task.uuid)
      ? { ...task, status: 'completed' }
      : task,
  )
}

interface LinkedTaskProps {
  task: Task
  now: number
  teller: Teller
}

// LinkedTask is one of the issue's tasks: its mark, id and description, how it
// stands, and — while it is still to do — the writes on it, named for it, since
// an issue can have several.
function LinkedTask({ task, now, teller }: LinkedTaskProps) {
  const number = taskNumber(task)

  return (
    <li className="flex flex-col gap-tight text-sm">
      <p className="flex items-start gap-item">
        <span className="flex h-5 shrink-0 items-center">
          <StateMark state={markOf(task)} className="text-taskwarrior" />
        </span>
        {number === '' ? null : (
          <span className="shrink-0 text-muted-foreground tabular-nums">#{number}</span>
        )}
        <span>{task.description}</span>
      </p>
      <Meta className="text-xs text-muted-foreground">{linkedNote(task, now)}</Meta>
      {stillToDo(task) ? (
        <div className="flex flex-wrap items-center gap-item">
          <TaskVerbs task={task} teller={teller} named={taskName(task)} />
        </div>
      ) : null}
    </li>
  )
}

// linkedNote is how a linked task stands: how long ago it was started, or its
// state and — for one still to do — when it is due.
function linkedNote(task: Task, now: number): string[] {
  if (isActive(task)) {
    return [`started ${elapsedWords(task.start, now)} ago`]
  }

  const state = statusWords(task)

  return stillToDo(task) && task.due !== undefined ? [state, dueWords(task.due, now)] : [state]
}

interface TrackIssueProps {
  issueKey: string
  // unlinked is whether no task is linked to the issue at all, which is said.
  unlinked: boolean
  teller: Teller
}

// TrackIssue tracks the issue in Taskwarrior — a task in the terminal's T line,
// annotated with the issue's page — and says which task now tracks it. Once
// the track has landed the page remembers the task it made, and the card
// draws neither the button nor the line saying no task tracks the issue until
// the stream holds it, so the issue is never tracked twice, nor said to be
// untracked; an answer that names no task takes them away here instead.
function TrackIssue({ issueKey, unlinked, teller }: TrackIssueProps) {
  const writes = useTaskWrites()
  const track = useAsyncAction(() => writes.track(issueKey), {
    fallback: `${issueKey} was not tracked. Try again, or track it from the terminal's Issues pane to see why.`,
    done: (answered) => trackedWords(answered, issueKey),
    onStart: teller.clear,
    onDone: teller.say,
  })
  const shortcut = useShortcutProps<HTMLButtonElement>('track-issue')

  if (track.state === 'done') {
    return null
  }

  return (
    <>
      {unlinked ? (
        <p className="text-sm text-muted-foreground">No task tracks {issueKey}.</p>
      ) : null}
      <div className="flex flex-wrap items-center gap-item">
        <Button
          {...shortcut}
          variant="secondary"
          held={track.state === 'running'}
          onClick={() => {
            void track.run()
          }}
        >
          {track.state === 'running' ? 'Tracking…' : 'Track in Taskwarrior'}
        </Button>
        {track.state === 'error' ? (
          <p role="alert" className="basis-full text-sm whitespace-pre-line text-destructive">
            {track.error}
          </p>
        ) : null}
      </div>
    </>
  )
}

// sentenceEnd matches words that already end a sentence.
const sentenceEnd = /[.!?]$/

// trackedWords says which task now tracks the issue, and what Taskwarrior had
// to add — that the task was made but its note of the issue's page was not —
// ending the sentence where Taskwarrior's words do not already.
function trackedWords(list: TaskList, issueKey: string): string {
  const tracks = `${capitalized(addedName(list))} tracks ${issueKey}`
  if (list.said === '') {
    return `${tracks}.`
  }

  return sentenceEnd.test(list.said) ? `${tracks}; ${list.said}` : `${tracks}; ${list.said}.`
}
