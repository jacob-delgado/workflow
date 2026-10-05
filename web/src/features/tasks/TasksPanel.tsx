import { useState, type ReactNode } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Issue, TaskList as Tasks } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { useNow } from './ActiveTask.tsx'
import { TaskDetail, Verb } from './TaskDetail.tsx'
import { TaskLineForm } from './TaskLineForm.tsx'
import { groupTasks, listedOf, TaskList, WaitingCount, type TaskGroups } from './TaskList.tsx'
import { TaskListControls } from './TaskListControls.tsx'
import { narrows } from './taskFacets.ts'
import { taskOrderWords, type TaskOrder } from './taskOrder.ts'
import { useTasks, useTaskWrites } from './tasksApi.ts'
import { addedName, addedTask, saidWords, taskName } from './taskWords.ts'

// TasksPanel is your pending Taskwarrior tasks — what the terminal's Tasks pane
// lists — beside the selected one's detail, under the line that adds one. It
// reads its own endpoint rather than the stream, as the section opens, and
// again when Refresh asks; each write answers the list after it, which takes
// the place of the one shown.
export function TasksPanel() {
  const query = useTasks()
  // A read after a failed first read is pending again, with no list yet; it is
  // a retry, which keeps the panel's place, and its control's focus, rather than
  // falling back to the first read's placeholder.
  const retrying = query.isPending && query.errorUpdateCount > 0

  if (query.isPending && !retrying) {
    return <EmptyState>Reading your tasks…</EmptyState>
  }

  if (query.data?.available === false) {
    return <Unavailable list={query.data} />
  }

  return (
    <Board
      list={query.data}
      failure={
        query.isError
          ? apiErrorMessage(query.error, 'Your tasks could not be read. Press Retry to try again.')
          : null
      }
      failed={query.isError || retrying}
      reading={query.isFetching}
      onReadAgain={() => {
        void query.refetch()
      }}
    />
  )
}

// Unavailable says why there is no Taskwarrior to ask, in the server's words,
// and — where the task on PATH is another program, go-task most likely — which
// setting finds Taskwarrior instead, which the server takes up when it next
// starts.
function Unavailable({ list }: { list: Tasks }) {
  return (
    <EmptyState>
      <span>
        {list.reason}
        {list.reason_code === 'not_taskwarrior' ? (
          <>
            {' '}
            Set <code className="font-mono">taskwarrior.program</code> in Settings, then restart
            workflow.
          </>
        ) : null}
      </span>
    </EmptyState>
  )
}

interface BoardProps {
  // list is the tasks as last read, or undefined when no read has landed.
  list: Tasks | undefined
  // failure is why the last read failed, while it stands.
  failure: string | null
  // failed is whether the last read failed, still so while a retry is in
  // flight.
  failed: boolean
  reading: boolean
  onReadAgain: () => void
}

// Board is the add line and the controls over the list and the selected task's
// detail, with the line that says what the last write did. A failed read leaves
// the list last read in view, beside its reason.
function Board({ list, failure, failed, reading, onReadAgain }: BoardProps) {
  const outcome = useOutcome()
  const now = useNow()
  const issues = useSnapshotStore((state) => state.snapshot?.issues.issues) ?? []
  const order = useUiStore((state) => state.taskOrder)
  const setOrder = useUiStore((state) => state.setTaskOrder)
  const picked = useUiStore((state) => state.taskFilter)
  const pick = useUiStore((state) => state.pickTaskFilter)
  const [text, setText] = useState('')
  const listing = { order, picked, text }
  const linked = new Set(issues.map((issue) => issue.key))
  const groups = list === undefined ? undefined : groupTasks(list.tasks, linked, now, listing)
  // The count a narrowed list is out of is what the list shows unnarrowed,
  // which leaves out the waiting tasks only a narrowing lists.
  const unnarrowed =
    list === undefined
      ? 0
      : listedOf(groupTasks(list.tasks, linked, now, { order, picked: [], text: '' })).length
  const narrowed = list !== undefined && list.tasks.length > 0 && narrows(listing)

  return (
    <div className="flex flex-col gap-group lg:min-h-0 lg:flex-1">
      <div className="flex flex-col gap-item">
        {list === undefined ? null : <AddLine teller={outcome} />}
        <Controls
          list={list}
          groups={groups}
          unnarrowed={unnarrowed}
          narrowed={narrowed}
          failure={failure}
          failed={failed}
          reading={reading}
          onReadAgain={onReadAgain}
          teller={outcome}
        />
        {list === undefined ? null : (
          <TaskListControls
            tasks={list.tasks}
            now={now}
            order={order}
            onOrder={setOrder}
            text={text}
            onText={setText}
            picked={picked}
            onPick={pick}
          />
        )}
        <OutcomeLine said={outcome.said} />
      </div>
      {groups === undefined ? null : (
        <Listing groups={groups} issues={issues} teller={outcome} now={now} narrowed={narrowed} />
      )}
    </div>
  )
}

// AddLine adds a task from a line in Taskwarrior's grammar, and says which task
// it added: by its id where the list holds it, and by the start of its uuid
// where the active context leaves it out.
function AddLine({ teller }: { teller: Teller }) {
  const writes = useTaskWrites()

  return (
    <TaskLineForm
      command="task add"
      verb="Add"
      busy="Adding…"
      send={writes.add}
      done={addedWords}
      fallback="Nothing was added. Try again, or run task add in a terminal to see why."
      teller={teller}
    />
  )
}

// addedWords says which task an add made, and what it is where the list shows
// it.
function addedWords(list: Tasks): string {
  const task = addedTask(list)

  return task === undefined
    ? `Added ${addedName(list)}.`
    : `Added ${taskName(task)}: ${task.description}`
}

interface ControlsProps extends BoardProps {
  groups: TaskGroups | undefined
  // narrowed is a list a filter or a narrowing leaves tasks out of.
  narrowed: boolean
  unnarrowed: number
  teller: Teller
}

// Controls are the lines that say how the list stands — how many it lists, the
// context narrowing it, and why the last read failed — beside the control that
// reads it again and the writes that name no task: undo, and sync where the
// taskrc names a backend. The read's control stays the same button whether it
// says Retry or Refresh, so the read it starts never takes its focus away.
function Controls({
  list,
  groups,
  narrowed,
  unnarrowed,
  failure,
  failed,
  reading,
  onReadAgain,
  teller,
}: ControlsProps) {
  const writes = useTaskWrites()

  return (
    <div className="flex flex-col gap-item">
      <p role="status" className="text-sm text-muted-foreground">
        {groups === undefined || list === undefined
          ? ''
          : listSummary(listedOf(groups).length, unnarrowed, groups.order, narrowed)}
      </p>
      {list === undefined || list.context === '' ? null : (
        <p className="text-sm text-muted-foreground">
          {`Taskwarrior's context ${list.context} narrows the list.`}
        </p>
      )}
      {failure === null ? null : (
        <p role="alert" className="text-sm whitespace-pre-line text-destructive">
          {failure}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-item">
        <Button
          variant="secondary"
          aria-disabled={reading}
          onClick={() => {
            if (!reading) {
              onReadAgain()
            }
          }}
        >
          {readAgainLabel(failed, reading)}
        </Button>
        {list === undefined ? null : (
          <Verb
            label="Undo"
            busy="Undoing…"
            run={writes.undo}
            done={(answered) => saidWords('Undone', answered.said)}
            fallback="Nothing was undone. Try again, or run task undo in a terminal to see why."
            teller={teller}
          />
        )}
        {list?.sync_available === true ? (
          <Verb
            label="Sync"
            busy="Syncing…"
            run={writes.sync}
            done={(answered) => saidWords('Synced', answered.said)}
            fallback="Taskwarrior did not sync. Try again, or run task sync in a terminal to see why."
            teller={teller}
          />
        ) : null}
      </div>
    </div>
  )
}

// listSummary says how many tasks the list shows. An empty list says so on
// screen below, in the list's place, so here it is said only to a screen
// reader.
function listSummary(
  listed: number,
  total: number,
  order: TaskOrder,
  narrowed: boolean,
): ReactNode {
  if (listed === 0) {
    return (
      <span className="sr-only">
        {narrowed ? 'No task matches the filters.' : 'No pending tasks.'}
      </span>
    )
  }

  const tasks = (count: number) => (count === 1 ? '1 task' : `${String(count)} tasks`)
  const how = taskOrderWords[order].toLowerCase()

  return narrowed
    ? `${String(listed)} of ${tasks(total)} match, ${how}.`
    : `${tasks(listed)}, ${how}.`
}

// readAgainLabel names the control that reads the list again: Retry after a
// failed read, Refresh otherwise, each saying so while the read is in flight.
function readAgainLabel(failed: boolean, reading: boolean): string {
  if (failed) {
    return reading ? 'Retrying…' : 'Retry'
  }

  return reading ? 'Refreshing…' : 'Refresh'
}

interface ListingProps {
  groups: TaskGroups
  // narrowed is a list read with tasks that the narrowing has emptied.
  narrowed: boolean
  // issues are those the Issues list holds, for the selected task's issue.
  issues: Issue[]
  teller: Teller
  now: number
}

// Listing is the list beside the selected task's detail — the first listed,
// until another is chosen — or, with none listed, how to get one. The task
// shown is held by its uuid, whether chosen or first, so a write that sorts the
// list anew keeps the detail on it; the first listed takes its place only once
// it leaves the list. Below lg the list sits over the detail, in a pane of a
// few rows, as the issue list does; from lg the two sit side by side, each
// scrolling on its own. The detail is keyed on its task, so a write that takes
// the task away takes its controls with it, rather than leaving them on the
// next task under the pointer.
function Listing({ groups, issues, teller, now, narrowed }: ListingProps) {
  const [held, setHeld] = useState<string | null>(null)
  const listed = listedOf(groups)
  const selected = listed.find((task) => task.uuid === held) ?? listed[0]
  if (selected !== undefined && selected.uuid !== held) {
    setHeld(selected.uuid)
  }

  if (selected === undefined) {
    return (
      <div className="flex flex-col gap-group">
        <EmptyState>
          {narrowed
            ? 'No task matches the filters.'
            : 'No pending tasks. Add one above, or track an issue from Issues.'}
        </EmptyState>
        <WaitingCount waiting={groups.waiting} />
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-block lg:min-h-0 lg:flex-1 lg:flex-row">
      <div className="relative flex max-h-80 min-h-0 flex-col gap-group overflow-y-auto rounded-lg border border-border p-1 lg:max-h-none lg:w-80 lg:shrink-0 lg:rounded-none lg:border-0">
        <TaskList groups={groups} selected={selected.uuid} onSelect={setHeld} now={now} />
      </div>
      <div className="relative min-w-0 flex-1 lg:overflow-y-auto lg:px-1">
        <TaskDetail
          key={selected.uuid}
          task={selected}
          issue={issues.find((issue) => issue.key === selected.issue_key)}
          teller={teller}
          now={now}
        />
      </div>
    </div>
  )
}
