import { ExternalLink } from 'lucide-react'
import { type ReactNode, useId, useState } from 'react'
import type { Issue, Task, TaskList } from '@/api/generated/types.gen.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import { writtenDate } from '@/lib/dates.ts'
import { Meta } from '@/lib/Meta.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { capitalized, definitionList } from '@/lib/utils.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { useTaskWrites } from './tasksApi.ts'
import { TaskLineForm } from './TaskLineForm.tsx'
import { dueWords, elapsedWords, isActive, statusWords, taskName, taskNumber } from './taskWords.ts'

const partHeading = 'text-base font-semibold'

interface TaskDetailProps {
  task: Task
  // issue is the task's issue as the Issues list holds it, or undefined where
  // it holds none.
  issue: Issue | undefined
  teller: Teller
  now: number
}

// TaskDetail is the selected task: what it is and its facts, the issue it is
// for, the notes on it, and the writes that change it — each saying what it did
// in the panel's outcome line, or why Taskwarrior refused it beside its control.
export function TaskDetail({ task, issue, teller, now }: TaskDetailProps) {
  const writes = useTaskWrites()
  const name = taskName(task)

  return (
    <article aria-labelledby="task-detail-heading" className="flex flex-col gap-block">
      <h2 id="task-detail-heading" className="text-lg">
        {task.description}
      </h2>
      <TaskFacts task={task} issue={issue} now={now} />
      <IssueLinks task={task} listed={issue !== undefined} />
      <div className="flex flex-wrap items-center gap-item">
        <TaskVerbs task={task} teller={teller} />
      </div>
      <Annotations task={task} />
      <div className="flex flex-col gap-group">
        <TaskLineForm
          shortcut="annotate-task"
          command={`${name} annotate`}
          verb="Annotate"
          busy="Annotating…"
          send={(text) => writes.annotate(task.uuid, text)}
          done={() => `Annotated ${name}.`}
          fallback={`${capitalized(name)} was not annotated. Try again, or run ${name} annotate in a terminal to see why.`}
          teller={teller}
        />
        <TaskLineForm
          shortcut="modify-task"
          command={`${name} modify`}
          verb="Modify"
          busy="Modifying…"
          send={(line) => writes.modify(task.uuid, line)}
          done={() => `Modified ${name}.`}
          fallback={`${capitalized(name)} was not modified. Try again, or run ${name} modify in a terminal to see why.`}
          teller={teller}
        />
      </div>
    </article>
  )
}

// TaskFacts are a task's facts, as the terminal's detail line has them: its
// state, project, priority, tags, due date and urgency, its id and its issue,
// leaving out any it does not have.
function TaskFacts({ task, issue, now }: Omit<TaskDetailProps, 'teller'>) {
  const number = taskNumber(task)

  return (
    <dl className={definitionList}>
      <Fact term="State">
        {isActive(task) ? `started ${elapsedWords(task.start, now)} ago` : statusWords(task)}
      </Fact>
      {task.project === '' ? null : <Fact term="Project">{task.project}</Fact>}
      {task.priority === '' ? null : <Fact term="Priority">{task.priority}</Fact>}
      {task.tags.length === 0 ? null : (
        <Fact term="Tags">{task.tags.map((tag) => `+${tag}`).join(' ')}</Fact>
      )}
      {task.due === undefined ? null : (
        <Fact term="Due">
          <Meta>
            <time dateTime={task.due}>{writtenDate(new Date(task.due))}</time>
            {dueWords(task.due, now)}
          </Meta>
        </Fact>
      )}
      <Fact term="Urgency">{task.urgency.toFixed(1)}</Fact>
      {number === '' ? null : <Fact term="ID">#{number}</Fact>}
      {task.issue_key === '' ? null : (
        <Fact term="Issue">
          <Meta>
            {task.issue_key}
            {issue?.summary}
          </Meta>
        </Fact>
      )}
    </dl>
  )
}

// Fact is one term of the facts and what it says.
function Fact({ term, children }: { term: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-muted-foreground">{term}</dt>
      <dd>{children}</dd>
    </>
  )
}

// IssueLinks open the task's issue: its page, in a new tab, where the task
// names one, and its detail in the Issues section, where the Issues list holds
// it — otherwise that section could not show it, as the terminal's "go to
// issue" is offered only then.
function IssueLinks({ task, listed }: { task: Task; listed: boolean }) {
  const setSection = useUiStore((state) => state.setSection)
  const selectIssue = useUiStore((state) => state.selectIssue)

  if (task.issue_url === '' && !listed) {
    return null
  }

  return (
    <div className="flex flex-wrap items-center gap-item text-sm">
      {task.issue_url === '' ? null : (
        <a
          href={task.issue_url}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1.5 text-primary underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          Open {task.issue_key}
          <ExternalLink aria-hidden className="size-3.5" />{' '}
          <span className="sr-only">(opens in a new tab)</span>
        </a>
      )}
      {listed ? (
        <Button
          variant="secondary"
          onClick={() => {
            selectIssue(task.issue_key)
            setSection('issues')
          }}
        >
          Open in Issues
        </Button>
      ) : null}
    </div>
  )
}

// Annotations are the notes on a task, oldest first, each after the day it was
// written, or nothing for a task with none.
function Annotations({ task }: { task: Task }) {
  if (task.annotations.length === 0) {
    return null
  }

  return (
    <div className="flex flex-col gap-group">
      <h3 id="task-notes-heading" className={partHeading}>
        Annotations
      </h3>
      <ul aria-labelledby="task-notes-heading" className="flex flex-col gap-tight text-sm">
        {task.annotations.map((note, index) => (
          <li key={`${String(index)}-${note.entry}`}>
            <time dateTime={note.entry} className="text-muted-foreground">
              {writtenDate(new Date(note.entry))}
            </time>{' '}
            {note.description}
          </li>
        ))}
      </ul>
    </div>
  )
}

interface TaskVerbsProps {
  task: Task
  teller: Teller
  // named is what a screen reader hears after each verb, naming the task where
  // several tasks' verbs stand together.
  named?: string
}

// TaskVerbs are the writes a button makes on one task: start it, or stop it
// once started, and mark it done. Standing alone, for the one task the Tasks
// section shows, they answer the terminal's s and d; where several tasks'
// verbs stand together, named, a key could not say which task it meant.
export function TaskVerbs({ task, teller, named }: TaskVerbsProps) {
  const writes = useTaskWrites()
  const name = taskName(task)
  const active = isActive(task)
  const alone = named === undefined

  return (
    <>
      <Verb
        label={active ? 'Stop' : 'Start'}
        busy={active ? 'Stopping…' : 'Starting…'}
        named={named}
        shortcut={alone ? 'start-stop' : undefined}
        run={() => (active ? writes.stop(task.uuid) : writes.start(task.uuid))}
        done={() => `${active ? 'Stopped' : 'Started'} ${name}.`}
        fallback={`${capitalized(name)} was not ${active ? 'stopped' : 'started'}. Try again, or run ${name} ${active ? 'stop' : 'start'} in a terminal to see why.`}
        teller={teller}
      />
      <Verb
        label="Mark done"
        busy="Marking done…"
        named={named}
        shortcut={alone ? 'mark-done' : undefined}
        ask={{
          question: `Mark ${name} done?`,
          cost: "Taskwarrior runs the task's hooks; only Undo, while it is the last change, takes it back.",
        }}
        run={() => writes.complete(task.uuid)}
        done={() => `Marked ${name} done.`}
        fallback={`${capitalized(name)} was not marked done. Try again, or run ${name} done in a terminal to see why.`}
        teller={teller}
      />
    </>
  )
}

// Ask is the last look a write waits on — a write that leaves the machine or
// cannot be taken back: the question it asks, and what going ahead costs.
interface Ask {
  question: string
  cost: string
}

interface VerbProps {
  label: string
  busy: string
  named?: string
  // shortcut is the terminal's action the button answers to, where it answers
  // one.
  shortcut?: string
  run: () => Promise<TaskList>
  done: (answered: TaskList) => string
  fallback: string
  teller: Teller
  // ask is the last look the write waits on, where it has one.
  ask?: Ask
}

// Verb is one write's button, and why Taskwarrior refused it — a change it made
// nothing of, say, which leaves the list as it was — drawn after every button
// of the row it stands in, so a refusal never parts a row's buttons. A write
// with a last look opens it first, as Forget… does, and sends only from it.
export function Verb({
  label,
  busy,
  named,
  shortcut,
  run,
  done,
  fallback,
  teller,
  ask,
}: VerbProps) {
  const [asking, setAsking] = useState(false)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const keys = useShortcut(shortcut, opener)
  const write = useAsyncAction(run, {
    fallback,
    done,
    onStart: teller.clear,
    onDone: (said) => {
      setAsking(false)
      teller.say(said)
    },
  })
  const running = write.state === 'running'
  const refusal =
    write.state === 'error' ? (
      <p
        role="alert"
        className="order-last basis-full text-sm whitespace-pre-line text-destructive"
      >
        {write.error}
      </p>
    ) : null

  if (ask !== undefined && asking) {
    return (
      <AskFirst
        ask={ask}
        onCancel={() => {
          write.reset()
          handBack()
          setAsking(false)
        }}
      >
        <Button variant="primary" disabled={running} onClick={() => void write.run()}>
          {running ? busy : label}
        </Button>
        {refusal}
      </AskFirst>
    )
  }

  return (
    <>
      <Button
        ref={opener}
        aria-keyshortcuts={keys}
        variant="secondary"
        disabled={running}
        onClick={() => {
          if (ask === undefined) {
            void write.run()
          } else {
            setAsking(true)
          }
        }}
      >
        {running ? busy : `${label}${ask === undefined ? '' : '…'}`}
        {named === undefined ? null : (
          <>
            {' '}
            <span className="sr-only">{named}</span>
          </>
        )}
      </Button>
      {refusal}
    </>
  )
}

// AskFirst is a write's last look: the question and its cost, Cancel, and the
// write's own button. It takes the focus as it opens, so a screen reader hears
// the question.
function AskFirst({
  ask,
  onCancel,
  children,
}: {
  ask: Ask
  onCancel: () => void
  children: ReactNode
}) {
  const question = useFocusOnMount<HTMLDivElement>()
  const questionId = useId()

  return (
    <div
      ref={question}
      role="group"
      aria-labelledby={questionId}
      tabIndex={-1}
      className="flex basis-full flex-col items-start gap-item text-sm"
    >
      <p id={questionId}>{ask.question}</p>
      <p className="text-muted-foreground">{ask.cost}</p>
      <div className="flex flex-wrap items-center gap-item">
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        {children}
      </div>
    </div>
  )
}
