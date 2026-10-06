import { useState } from 'react'
import type { IssueDetail } from '@/api/generated/types.gen.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { Input, TextArea } from '@/lib/Field.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { shownKey } from '../issuePlaces.ts'
import { useIssueWrites } from './issueWritesApi.ts'
import { StatusChangeForm } from './StatusChangeForm.tsx'
import { LabeledInput, WriteForm } from '@/lib/WriteForm.tsx'

type Opened = 'none' | 'status' | 'assign' | 'worklog'

// IssueActions changes an issue from its detail, as the terminal's t, a and w
// do: its status, with the fields the change needs; whom it is assigned to;
// and the work logged on it, which only a Jira issue keeps. Each opens a form
// that is the last look at what goes to the tracker, and sends only from it.
// One form is open at a time; closing it hands focus back to its button, and
// what a sent one did is said in the line under the buttons.
export function IssueActions({ detail }: { detail: IssueDetail }) {
  const [opened, setOpened] = useState<Opened>('none')
  const outcome = useOutcome()
  const [statusButton, statusBack] = useFocusHandback<HTMLButtonElement>()
  const [assignButton, assignBack] = useFocusHandback<HTMLButtonElement>()
  const [worklogButton, worklogBack] = useFocusHandback<HTMLButtonElement>()
  const statusKeys = useShortcut('change-status', statusButton)
  const assignKeys = useShortcut('assign', assignButton)
  const worklogKeys = useShortcut('log-work', worklogButton)
  const key = detail.key
  const shown = shownKey({ key, tracker: detail.tracker })
  const handBack = { status: statusBack, assign: assignBack, worklog: worklogBack, none: () => {} }
  const close = () => {
    handBack[opened]()
    setOpened('none')
  }
  const teller: Teller = {
    clear: outcome.clear,
    say: (said) => {
      setOpened('none')
      outcome.say(said)
    },
  }

  return (
    <div className="flex flex-col gap-item">
      {opened === 'none' ? (
        <div role="group" aria-label={`Change ${shown}`} className="flex flex-wrap gap-item">
          <Button
            variant="secondary"
            ref={statusButton}
            aria-keyshortcuts={statusKeys}
            onClick={() => {
              setOpened('status')
            }}
          >
            Change status
          </Button>
          <Button
            variant="secondary"
            ref={assignButton}
            aria-keyshortcuts={assignKeys}
            onClick={() => {
              setOpened('assign')
            }}
          >
            Assign
          </Button>
          {detail.tracker === 'jira' ? (
            <Button
              variant="secondary"
              ref={worklogButton}
              aria-keyshortcuts={worklogKeys}
              onClick={() => {
                setOpened('worklog')
              }}
            >
              Log work
            </Button>
          ) : null}
        </div>
      ) : null}
      {opened === 'status' ? (
        <StatusChangeForm issueKey={key} shown={shown} teller={teller} onCancel={close} />
      ) : null}
      {opened === 'assign' ? (
        <AssignForm issueKey={key} shown={shown} teller={teller} onCancel={close} />
      ) : null}
      {opened === 'worklog' ? (
        <WorklogForm issueKey={key} shown={shown} teller={teller} onCancel={close} />
      ) : null}
      <OutcomeLine said={outcome.said} />
    </div>
  )
}

interface WriteFormProps {
  issueKey: string
  shown: string
  teller: Teller
  onCancel: () => void
}

// AssignForm assigns the issue to the username typed: a Jira username on a
// Jira issue, the forge's on a forge issue.
function AssignForm({ issueKey, shown, teller, onCancel }: WriteFormProps) {
  const [assignee, setAssignee] = useState('')
  const writes = useIssueWrites(issueKey)
  const assign = useAsyncAction(writes.assign, {
    fallback: `${shown} could not be assigned. Try again, or assign it in its tracker.`,
    done: (assigned) => `Assigned ${shown} to ${assigned.assignee}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <WriteForm
      label={`Assign ${shown}`}
      act="Assign"
      busy={assign.state === 'running' ? 'Assigning…' : null}
      error={assign.state === 'error' ? assign.error : ''}
      onSend={() => void assign.run(assignee)}
      onCancel={onCancel}
    >
      <LabeledInput label="Assignee" hint="A username, as the tracker knows it.">
        {(id, hint) => (
          <Input
            id={id}
            aria-describedby={hint}
            value={assignee}
            autoComplete="off"
            required
            onChange={(event) => {
              setAssignee(event.target.value)
            }}
          />
        )}
      </LabeledInput>
    </WriteForm>
  )
}

// WorklogForm logs time spent on a Jira issue, with an optional note.
function WorklogForm({ issueKey, shown, teller, onCancel }: WriteFormProps) {
  const [spent, setSpent] = useState('')
  const [note, setNote] = useState('')
  const writes = useIssueWrites(issueKey)
  const log = useAsyncAction(writes.logWork, {
    fallback: `The work could not be logged on ${shown}. Try again, or log it in Jira.`,
    done: (logged) => `Logged ${logged.time_spent} on ${shown}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <WriteForm
      label={`Log work on ${shown}`}
      act="Log work"
      busy={log.state === 'running' ? 'Logging…' : null}
      error={log.state === 'error' ? log.error : ''}
      onSend={() => void log.run(spent, note)}
      onCancel={onCancel}
    >
      <LabeledInput label="Time spent" hint="In Jira's words, such as 2h, 30m or 1d 4h.">
        {(id, hint) => (
          <Input
            id={id}
            aria-describedby={hint}
            value={spent}
            autoComplete="off"
            required
            onChange={(event) => {
              setSpent(event.target.value)
            }}
          />
        )}
      </LabeledInput>
      <LabeledInput label="Note" hint="Optional.">
        {(id, hint) => (
          <TextArea
            id={id}
            aria-describedby={hint}
            rows={2}
            value={note}
            onChange={(event) => {
              setNote(event.target.value)
            }}
          />
        )}
      </LabeledInput>
    </WriteForm>
  )
}
