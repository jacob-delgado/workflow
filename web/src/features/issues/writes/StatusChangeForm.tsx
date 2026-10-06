import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { FieldEntry, StatusChange, StatusChangeField } from '@/api/generated/types.gen.ts'
import { Input, Select } from '@/lib/Field.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { Reading, Unread } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { useIssueWrites, useStatusChanges } from './issueWritesApi.ts'
import { LabeledInput, WriteForm } from '@/lib/WriteForm.tsx'

interface StatusChangeFormProps {
  issueKey: string
  shown: string
  teller: Teller
  onCancel: () => void
}

// Filled is what has been entered for each field of the chosen change, by the
// field's id: the typed text or the chosen option, or the options checked.
type Filled = Record<string, string | string[]>

// StatusChangeForm changes the issue's status, as the terminal's t and its
// field form do: it lists the changes the tracker offers from where the issue
// stands, and once one is chosen, the fields that change needs. A change
// needing a field only Jira's own screen can fill says so and is not offered
// to send. What is sent is checked by the server, whose refusal stays beside
// the form.
export function StatusChangeForm({ issueKey, shown, teller, onCancel }: StatusChangeFormProps) {
  const [chosen, setChosen] = useState<StatusChange | null>(null)
  const [filled, setFilled] = useState<Filled>({})
  const writes = useIssueWrites(issueKey)
  const change = useAsyncAction(writes.changeStatus, {
    fallback: `The status of ${shown} could not be changed. Try again, or change it in its tracker.`,
    done: (moved) => `Changed ${shown} to ${moved.status}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })
  const blocked = chosen === null ? undefined : onlyJira(chosen)

  return (
    <WriteForm
      label={`Change the status of ${shown}`}
      act={chosen === null ? 'Change status' : `Change to ${chosen.to_status}`}
      busy={change.state === 'running' ? 'Changing…' : null}
      error={change.state === 'error' ? change.error : ''}
      disabled={chosen === null || blocked !== undefined}
      onSend={() => {
        if (chosen !== null) {
          void change.run(chosen, entriesOf(chosen, filled))
        }
      }}
      onCancel={onCancel}
    >
      <OfferedChanges
        issueKey={issueKey}
        shown={shown}
        chosen={chosen}
        onChoose={(next) => {
          setChosen(next)
          setFilled({})
          change.reset()
        }}
      />
      {chosen === null ? null : (
        <ChosenFields
          chosen={chosen}
          filled={filled}
          onFill={(fieldID, value) => {
            setFilled((before) => ({ ...before, [fieldID]: value }))
          }}
        />
      )}
    </WriteForm>
  )
}

// OfferedChanges reads the changes the tracker offers the issue and offers
// each, saying so while it reads, when the read fails, and when there are none.
function OfferedChanges({
  issueKey,
  shown,
  chosen,
  onChoose,
}: {
  issueKey: string
  shown: string
  chosen: StatusChange | null
  onChoose: (change: StatusChange) => void
}) {
  const changes = useStatusChanges(issueKey)

  if (changes.isPending) {
    return <Reading>Reading the status changes of {shown}…</Reading>
  }

  if (changes.isError) {
    return (
      <Unread
        reason={apiErrorMessage(changes.error, `The status changes of ${shown} could not be read.`)}
        refusals={changes.errorUpdateCount}
        retrying={changes.isFetching}
        onRetry={() => {
          void changes.refetch()
        }}
      />
    )
  }

  if (changes.data.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        The tracker offers no status change for {shown}.
      </p>
    )
  }

  return <ChangeChoice changes={changes.data} chosen={chosen} onChoose={onChoose} />
}

// ChosenFields is the form of the change chosen: a field for each field it
// needs, or why it cannot be made here when one only Jira's own screen fills.
function ChosenFields({
  chosen,
  filled,
  onFill,
}: {
  chosen: StatusChange
  filled: Filled
  onFill: (fieldID: string, value: string | string[]) => void
}) {
  const blocked = onlyJira(chosen)
  if (blocked !== undefined) {
    return (
      <p role="note" className="text-sm text-muted-foreground">
        {chosen.name} needs {blocked.name}, which only Jira&apos;s own screen can fill; make this
        change in Jira.
      </p>
    )
  }

  return chosen.fields.map((field) => (
    <FieldInput
      key={field.id}
      field={field}
      value={filled[field.id]}
      onChange={(value) => {
        onFill(field.id, value)
      }}
    />
  ))
}

// ChangeChoice offers each change by where it leads, naming the change when
// its name differs and the fields it needs, so none is a surprise.
function ChangeChoice({
  changes,
  chosen,
  onChoose,
}: {
  changes: StatusChange[]
  chosen: StatusChange | null
  onChoose: (change: StatusChange) => void
}) {
  return (
    <LabeledInput label="New status">
      {(id) => (
        <Select
          id={id}
          value={chosen?.id ?? ''}
          onChange={(event) => {
            const next = changes.find((change) => change.id === event.target.value)
            if (next !== undefined) {
              onChoose(next)
            }
          }}
        >
          <option value="" disabled>
            Choose a status…
          </option>
          {changes.map((change) => (
            <option key={change.id} value={change.id}>
              {change.to_status}
              {describe(change)}
            </option>
          ))}
        </Select>
      )}
    </LabeledInput>
  )
}

// describe says what a change is called, when that is not where it leads, and
// what it needs.
function describe(change: StatusChange): string {
  const named = change.name === change.to_status ? '' : ` (${change.name})`
  const needs =
    change.fields.length === 0
      ? ''
      : ` — needs ${change.fields.map((field) => field.name).join(', ')}`

  return named + needs
}

// FieldInput fills one field a change needs: a choice of one of its values, a
// box per value for a list, a date, or typed text or a username.
function FieldInput({
  field,
  value,
  onChange,
}: {
  field: StatusChangeField
  value: string | string[] | undefined
  onChange: (value: string | string[]) => void
}) {
  if (field.kind === 'option_list') {
    const checked = Array.isArray(value) ? value : []

    return (
      <fieldset className="flex flex-col gap-tight">
        <legend className="mb-tight text-sm font-medium">{field.name}</legend>
        {field.options.map((option) => (
          <label key={option.id} className="flex items-center gap-item text-sm">
            <input
              type="checkbox"
              checked={checked.includes(option.id)}
              onChange={() => {
                onChange(toggled(checked, option.id))
              }}
              className="focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
            />
            {option.name}
          </label>
        ))}
      </fieldset>
    )
  }

  const text = typeof value === 'string' ? value : ''

  return (
    <LabeledInput label={field.name} hint={hints[field.kind]}>
      {(id, hint) =>
        field.kind === 'option' ? (
          <Select
            id={id}
            value={text}
            onChange={(event) => {
              onChange(event.target.value)
            }}
          >
            <option value="">Choose…</option>
            {field.options.map((option) => (
              <option key={option.id} value={option.id}>
                {option.name}
              </option>
            ))}
          </Select>
        ) : (
          <Input
            id={id}
            aria-describedby={hint}
            autoComplete="off"
            value={text}
            onChange={(event) => {
              onChange(event.target.value)
            }}
          />
        )
      }
    </LabeledInput>
  )
}

// hints say what a typed field takes, as the terminal's placeholders do.
const hints: Partial<Record<StatusChangeField['kind'], string>> = {
  date: 'A date, such as 2026-09-21.',
  user: 'A username, as the tracker knows it.',
}

// toggled is ids with id added when absent, and taken out when present.
function toggled(ids: string[], id: string): string[] {
  return ids.includes(id) ? ids.filter((each) => each !== id) : [...ids, id]
}

// onlyJira is the first field of a change only Jira's own screen can fill.
function onlyJira(change: StatusChange): StatusChangeField | undefined {
  return change.fields.find((field) => field.kind === 'only_jira')
}

// entriesOf is what is sent for each field change needs, in its order: the
// options checked for a list, the option chosen for an option field, and the
// text otherwise. A field left empty is sent empty, for the server to name.
function entriesOf(change: StatusChange, filled: Filled): FieldEntry[] {
  return change.fields.map((field) => {
    const value = filled[field.id]
    if (field.kind === 'option_list') {
      return { id: field.id, option_ids: Array.isArray(value) ? value : [] }
    }

    const text = typeof value === 'string' ? value : ''

    return field.kind === 'option' ? { id: field.id, option_id: text } : { id: field.id, text }
  })
}
