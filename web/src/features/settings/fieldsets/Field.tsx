import { useId, type ReactNode } from 'react'
import { get, type Path, useFormContext, type UseFormRegister } from 'react-hook-form'
import { Input, Select } from '@/lib/Field.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import type { SettingsValues } from '../formValues.ts'
import type { CredentialPath } from '../removal.ts'
import { ruleFor } from '../settingsRules.ts'
import { CredentialRemoval } from './CredentialRemoval.tsx'

// Register is the settings form's own register, which each fieldset is handed
// to put its fields in the form.
export type Register = UseFormRegister<SettingsValues>

// A field's name is its path in the configuration, which is also its element's
// id, so a label, a hint and the saved value all agree on what it is.
type Name = Path<SettingsValues>

// A field that reads its value as something other than the text typed.
type ReadAs = (value: unknown) => unknown

// readCount reads a count typed into a number field: empty is zero, which the
// configuration takes as "keep the default".
function readCount(value: unknown): unknown {
  return value === '' || value === null ? 0 : Number(value)
}

// hintId is the id of the hint that describes a field, for its control's
// aria-describedby.
function hintId(name: Name): string {
  return `${name}-hint`
}

// refusalId is the id of what says why a field was refused.
function refusalId(name: Name): string {
  return `${name}-refusal`
}

// useRefusal is why the form refuses a field, by the page's rules or the
// server's, or undefined while it does not.
function useRefusal(name: Name): string | undefined {
  const { formState } = useFormContext<SettingsValues>()
  const refused: unknown = get(formState.errors, name)

  return typeof refused === 'object' &&
    refused !== null &&
    'message' in refused &&
    typeof refused.message === 'string'
    ? refused.message
    : undefined
}

// described are the attributes that tie a control to its hint and to why it
// was refused, and mark it refused while it is.
function described(name: Name, hint: boolean, refusal: string | undefined) {
  const ids = [hint ? hintId(name) : '', refusal === undefined ? '' : refusalId(name)].filter(
    (id) => id !== '',
  )

  return {
    'aria-describedby': ids.length === 0 ? undefined : ids.join(' '),
    'aria-invalid': refusal === undefined ? undefined : true,
  }
}

interface FieldsetProps {
  legend: string
  // hint is what holds for every field in the section, below its legend.
  hint?: string
  children: ReactNode
}

// Fieldset is one section of the configuration, headed by what it configures,
// with the hint that describes the whole of it below.
export function Fieldset({ legend, hint, children }: FieldsetProps) {
  const sectionHintId = useId()

  return (
    <fieldset
      aria-describedby={hint ? sectionHintId : undefined}
      className="flex flex-col gap-group"
    >
      <legend className="mb-group text-base font-semibold">{legend}</legend>
      {hint ? (
        <p id={sectionHintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
      {children}
    </fieldset>
  )
}

interface FieldProps {
  name: Name
  label: string
  hint?: string
  refusal?: string
  children: ReactNode
}

// Field is a control under its label, with why it was refused, while it is,
// and the hint that describes it below.
function Field({ name, label, hint, refusal, children }: FieldProps) {
  return (
    <div className="flex flex-col gap-tight">
      <label htmlFor={name} className="text-sm text-muted-foreground">
        {label}
      </label>
      {children}
      {refusal === undefined ? null : (
        <p id={refusalId(name)} className="text-xs text-destructive">
          {refusal}
        </p>
      )}
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
  // readOnly shows a setting a save keeps as the file holds it: it rides
  // back as it was read, and is not typed into.
  readOnly?: boolean
}

// TextField is a field typed into: text, a URL, a secret, or a count, which
// is never below zero and reads as a number.
export function TextField({
  register,
  name,
  label,
  hint,
  type = 'text',
  readAs,
  readOnly = false,
}: TextFieldProps) {
  const refusal = useRefusal(name)

  return (
    <Field name={name} label={label} hint={hint} refusal={refusal}>
      <Input
        id={name}
        type={type}
        min={type === 'number' ? 0 : undefined}
        readOnly={readOnly}
        {...described(name, hint !== undefined, refusal)}
        {...register(name, {
          setValueAs: type === 'number' ? readCount : readAs,
          validate: ruleFor(name),
        })}
      />
    </Field>
  )
}

interface SecretFieldProps {
  register: Register
  name: CredentialPath
  label: string
  hint: string
}

// SecretField is a credential: typed into a field that never shows it, and,
// beside it while the file holds one, Remove…, which takes it out of the file.
export function SecretField({ register, name, label, hint }: SecretFieldProps) {
  const outcome = useOutcome()
  const refusal = useRefusal(name)

  return (
    <Field name={name} label={label} hint={hint} refusal={refusal}>
      <div className="flex flex-wrap items-center gap-item">
        <Input
          id={name}
          type="password"
          className="min-w-0 flex-1"
          {...described(name, true, refusal)}
          {...register(name)}
        />
        <CredentialRemoval removal={{ credential: name }} tell={outcome} />
      </div>
      <OutcomeLine said={outcome.said} />
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
  const refusal = useRefusal(name)

  return (
    <Field name={name} label={label} hint={hint} refusal={refusal}>
      <Select id={name} {...described(name, hint !== undefined, refusal)} {...register(name)}>
        {choices.map(([value, words]) => (
          <option key={value} value={value}>
            {words}
          </option>
        ))}
      </Select>
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
