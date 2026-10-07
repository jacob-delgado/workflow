import { useId, useRef } from 'react'
import {
  type ArrayPath,
  type Control,
  type FieldArray,
  type Path,
  get,
  useFieldArray,
  useFormState,
} from 'react-hook-form'
import { Button } from '@/lib/Button.tsx'
import { Input } from '@/lib/Field.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { cn } from '@/lib/utils.ts'
import type { SettingsValues } from '../formValues.ts'
import type { Removal } from '../removal.ts'
import { CredentialRemoval } from './CredentialRemoval.tsx'
import type { Register } from './Field.tsx'

// A list the form edits row by row.
type ListName = ArrayPath<SettingsValues>

// Column is one field of a row: the row's key it edits, the words it is
// labeled with, whether it holds a credential, and whether it takes the room
// the others leave.
interface Column {
  field: string
  label: string
  secret?: boolean
  wide?: boolean
}

// Unique is how a list's rows are told apart by name, as the configuration
// matches them: namesOf reads every row's name from the form, key is the form
// two names that are one share, and taken says why a later row is refused.
export interface Unique {
  namesOf: (values: SettingsValues) => string[]
  key: (name: string) => string
  taken: string
}

// matchedLower is the key of a name matched without regard to case or the
// space around it: a Jira header, and an issue type.
export function matchedLower(name: string): string {
  return name.trim().toLowerCase()
}

// takenBy is what refuses a row whose name an earlier row already has, for
// the row at index; a row without a name is not refused here.
function takenBy(unique: Unique, index: number) {
  return (value: unknown, values: SettingsValues): true | string => {
    const key = unique.key(String(value))
    const earlier = unique.namesOf(values).slice(0, index)

    return key === '' || !earlier.some((name) => unique.key(name) === key) ? true : unique.taken
  }
}

interface EntryListProps<N extends ListName> {
  control: Control<SettingsValues>
  register: Register
  name: N
  // legend names the list, and entry one row of it: "Views", "view".
  legend: string
  entry: string
  hint?: string
  columns: Column[]
  // blank is a new row, as Add puts it in the list.
  blank: FieldArray<SettingsValues, N>
  // storedAs is what removing a row the file holds takes out of it, or null
  // for a row it does not hold: a stored header, whose name is not edited —
  // the mask of its value is tied to it — and whose removal is asked first and
  // written at once, since its value is a credential.
  storedAs?: (index: number) => Removal | null
  // unique, for a list whose names are keys, refuses a row named as an
  // earlier one at its name, before anything is sent.
  unique?: Unique
}

// EntryList is a list in the configuration edited as rows — the views, the
// channels, the branch prefixes, the Jira headers: each row a group named for
// its place in the list, holding its fields and its Remove, and Add below the
// rows. Removing a row is an edit like any other, saved with the form; focus
// goes to Add, since the Remove pressed is gone.
export function EntryList<N extends ListName>({
  control,
  register,
  name,
  legend,
  entry,
  hint,
  columns,
  blank,
  storedAs = () => null,
  unique,
}: EntryListProps<N>) {
  const { fields, append, remove } = useFieldArray({ control, name })
  const { errors } = useFormState({ control, name })
  const outcome = useOutcome()
  const add = useRef<HTMLButtonElement>(null)
  const hintId = useId()
  const Entry = entry.charAt(0).toUpperCase() + entry.slice(1)

  return (
    <fieldset
      aria-describedby={hint ? hintId : undefined}
      className="flex min-w-0 flex-col gap-tight"
    >
      <legend className="text-sm text-muted-foreground">{legend}</legend>
      {hint ? (
        <p id={hintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
      {fields.map((row, index) => {
        // The name is the row's first field, and the one a refusal is about.
        const namePath = `${name}.${String(index)}.${columns[0]?.field ?? ''}`
        const refusal = get(errors, `${namePath}.message`) as string | undefined
        const refusalId = `${hintId}-${namePath}`

        return (
          <div
            key={row.id}
            role="group"
            aria-label={`${Entry} ${String(index + 1)}`}
            className="flex flex-wrap items-center gap-item"
          >
            <RowFields
              register={register}
              row={`${name}.${String(index)}`}
              columns={columns}
              nameHeld={storedAs(index) !== null}
              refusalId={refusal === undefined ? undefined : refusalId}
              validate={unique ? takenBy(unique, index) : undefined}
            />
            <RowRefusal id={refusalId} refusal={refusal} />
            <RowRemoval
              removal={storedAs(index)}
              label={`Remove ${entry} ${String(index + 1)}`}
              tell={outcome}
              onRemove={() => {
                remove(index)
                add.current?.focus()
              }}
            />
          </div>
        )
      })}
      <Button
        ref={add}
        variant="secondary"
        size="sm"
        className="self-start"
        onClick={() => {
          append(blank)
        }}
      >
        Add a {entry}
      </Button>
      <OutcomeLine said={outcome.said} />
    </fieldset>
  )
}

interface RowFieldsProps {
  register: Register
  // row is the row's path in the form, as "jira.headers.1".
  row: string
  columns: Column[]
  // nameHeld keeps the name from being edited; refusalId names the refusal
  // the name is described by, while there is one; validate refuses a name.
  nameHeld: boolean
  refusalId: string | undefined
  validate: ((value: unknown, values: SettingsValues) => true | string) | undefined
}

// RowFields are a row's fields, its name first.
function RowFields({ register, row, columns, nameHeld, refusalId, validate }: RowFieldsProps) {
  return columns.map((column, at) => (
    <Input
      key={column.field}
      aria-label={column.label}
      placeholder={column.label}
      type={column.secret ? 'password' : 'text'}
      readOnly={at === 0 && nameHeld}
      aria-invalid={at === 0 && refusalId !== undefined ? true : undefined}
      aria-describedby={at === 0 ? refusalId : undefined}
      className={cn(column.wide ? 'min-w-48 flex-[2]' : 'min-w-32 flex-1')}
      {...register(
        `${row}.${column.field}` as Path<SettingsValues>,
        at === 0 && validate ? { validate } : undefined,
      )}
    />
  ))
}

// RowRefusal says why a row's name is refused, beneath the row, where the
// name it describes is.
function RowRefusal({ id, refusal }: { id: string; refusal: string | undefined }) {
  if (refusal === undefined) {
    return null
  }

  return (
    <p id={id} role="alert" className="order-last basis-full text-sm text-destructive">
      {refusal}
    </p>
  )
}

interface RowRemovalProps {
  removal: Removal | null
  label: string
  tell: ReturnType<typeof useOutcome>
  onRemove: () => void
}

// RowRemoval is a row's Remove: an edit saved with the form, or, for a row
// the file holds a credential in, the removal asked first and written at once.
function RowRemoval({ removal, label, tell, onRemove }: RowRemovalProps) {
  if (removal !== null) {
    return <CredentialRemoval removal={removal} tell={tell} />
  }

  return (
    <Button variant="secondary" size="sm" aria-label={label} onClick={onRemove}>
      Remove
    </Button>
  )
}
