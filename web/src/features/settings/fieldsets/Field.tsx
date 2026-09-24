import type { ReactNode } from 'react'
import type { Path, UseFormRegister } from 'react-hook-form'
import type { Config } from '@/api/generated/types.gen.ts'

// Register is the settings form's own register, which each fieldset is handed
// to put its fields in the form.
export type Register = UseFormRegister<Config>

// A field's name is its path in the configuration, which is also its element's
// id, so a label, a hint and the saved value all agree on what it is.
type Name = Path<Config>

// A field that reads its value as something other than the text typed.
type ReadAs = (value: unknown) => unknown

// readCount reads a count typed into a number field: empty is zero, which the
// configuration takes as "keep the default".
function readCount(value: unknown): unknown {
  return value === '' || value === null ? 0 : Number(value)
}

const inputClass =
  'rounded-md border border-input bg-transparent px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'

// hintId is the id of the hint that describes a field, for its control's
// aria-describedby.
function hintId(name: Name): string {
  return `${name}-hint`
}

// Fieldset is one section of the configuration, headed by what it configures.
export function Fieldset({ legend, children }: { legend: string; children: ReactNode }) {
  return (
    <fieldset className="flex flex-col gap-group">
      <legend className="mb-group text-base font-semibold">{legend}</legend>
      {children}
    </fieldset>
  )
}

interface FieldProps {
  name: Name
  label: string
  hint?: string
  children: ReactNode
}

// Field is a control under its label, with the hint that describes it below.
function Field({ name, label, hint, children }: FieldProps) {
  return (
    <div className="flex flex-col gap-tight">
      <label htmlFor={name} className="text-sm text-muted-foreground">
        {label}
      </label>
      {children}
      {hint ? (
        <p id={hintId(name)} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  )
}

interface TextFieldProps {
  register: Register
  name: Name
  label: string
  hint?: string
  type?: 'text' | 'url' | 'password' | 'number'
  readAs?: ReadAs
}

// TextField is a field typed into: text, a URL, a secret, or a count, which
// is never below zero and reads as a number.
export function TextField({ register, name, label, hint, type = 'text', readAs }: TextFieldProps) {
  return (
    <Field name={name} label={label} hint={hint}>
      <input
        id={name}
        type={type}
        min={type === 'number' ? 0 : undefined}
        aria-describedby={hint ? hintId(name) : undefined}
        className={inputClass}
        {...register(name, { setValueAs: type === 'number' ? readCount : readAs })}
      />
    </Field>
  )
}

interface SelectFieldProps {
  register: Register
  name: Name
  label: string
  hint?: string
  // choices are the values offered, each beside the words it is offered in.
  choices: [value: string, words: string][]
}

// SelectField is a field chosen from a fixed set.
export function SelectField({ register, name, label, hint, choices }: SelectFieldProps) {
  return (
    <Field name={name} label={label} hint={hint}>
      <select
        id={name}
        aria-describedby={hint ? hintId(name) : undefined}
        className={inputClass}
        {...register(name)}
      >
        {choices.map(([value, words]) => (
          <option key={value} value={value}>
            {words}
          </option>
        ))}
      </select>
    </Field>
  )
}

// CheckboxField is a setting that is on or off, named by what it turns on.
export function CheckboxField({
  register,
  name,
  label,
}: {
  register: Register
  name: Name
  label: string
}) {
  return (
    <label className="flex items-center gap-2 text-sm">
      <input type="checkbox" className="size-4" {...register(name)} />
      {label}
    </label>
  )
}
