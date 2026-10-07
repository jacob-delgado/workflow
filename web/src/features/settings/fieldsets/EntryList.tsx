import { useId, useRef } from 'react'
import {
  type ArrayPath,
  type Control,
  type FieldArray,
  type Path,
  useFieldArray,
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
}: EntryListProps<N>) {
  const { fields, append, remove } = useFieldArray({ control, name })
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
      {fields.map((row, index) => (
        <div
          key={row.id}
          role="group"
          aria-label={`${Entry} ${String(index + 1)}`}
          className="flex flex-wrap items-center gap-item"
        >
          {columns.map((column, at) => (
            <Input
              key={column.field}
              aria-label={column.label}
              placeholder={column.label}
              type={column.secret ? 'password' : 'text'}
              readOnly={at === 0 && storedAs(index) !== null}
              className={cn(column.wide ? 'min-w-48 flex-[2]' : 'min-w-32 flex-1')}
              {...register(`${name}.${String(index)}.${column.field}` as Path<SettingsValues>)}
            />
          ))}
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
      ))}
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
