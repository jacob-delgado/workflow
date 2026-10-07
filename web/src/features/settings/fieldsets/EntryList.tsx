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
import { cn } from '@/lib/utils.ts'
import type { SettingsValues } from '../formValues.ts'
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
  // fixed reports a row whose first field is not edited: a stored header,
  // whose name the mask of its value is tied to.
  fixed?: (index: number) => boolean
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
  fixed = () => false,
}: EntryListProps<N>) {
  const { fields, append, remove } = useFieldArray({ control, name })
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
              readOnly={at === 0 && fixed(index)}
              className={cn(column.wide ? 'min-w-48 flex-[2]' : 'min-w-32 flex-1')}
              {...register(`${name}.${String(index)}.${column.field}` as Path<SettingsValues>)}
            />
          ))}
          <Button
            variant="secondary"
            size="sm"
            aria-label={`Remove ${entry} ${String(index + 1)}`}
            onClick={() => {
              remove(index)
              add.current?.focus()
            }}
          >
            Remove
          </Button>
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
    </fieldset>
  )
}
