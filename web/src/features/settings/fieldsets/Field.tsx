import { useId, useState, type ReactNode } from 'react'
import { get, type Path, type UseFormRegister, useFormState } from 'react-hook-form'
import { Button } from '@/lib/Button.tsx'
import { Input, Select } from '@/lib/Field.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import { LastLook } from '@/lib/LastLook.tsx'
import { OutcomeLine, type Teller, useOutcome } from '@/lib/Outcome.tsx'
import { Failure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import type { SettingsValues } from '../formValues.ts'
import { ruleFor } from '../settingsRules.ts'
import {
  type CredentialPath,
  type Removal,
  removalConsequence,
  removalWords,
  useRemovals,
} from '../removal.ts'

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

// refusalId is the id of what says why a field is refused.
function refusalId(name: Name): string {
  return `${name}-refusal`
}

// useRefusal is why the form refuses a field — a rule's word before a save,
// or the server's after one — or undefined while it is not refused.
function useRefusal(name: Name): string | undefined {
  const { errors } = useFormState<SettingsValues>({ name })

  return get(errors, `${name}.message`) as string | undefined
}

// keptId is the id of what says where a credential is kept.
function keptId(name: Name): string {
  return `${name}-kept`
}

// Described is what a field's control is described by: where its credential
// is kept, its hint and why it is refused, each while it has one.
interface Described {
  kept?: string
  hint?: string
  refusal?: string
}

// describedProps are what a field's control says of what describes it, and
// that it is invalid while it is refused.
function describedProps(name: Name, { kept, hint, refusal }: Described) {
  const describers = [
    kept === undefined ? '' : keptId(name),
    hint ? hintId(name) : '',
    refusal === undefined ? '' : refusalId(name),
  ]
  const describedBy = describers.filter((id) => id !== '').join(' ')

  return {
    'aria-describedby': describedBy === '' ? undefined : describedBy,
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

interface FieldProps extends Described {
  name: Name
  label: string
  children: ReactNode
}

// Field is a control under its label — and, for a credential, where it is
// kept — with the hint that describes it below, and below that why it is
// refused, while it is.
function Field({ name, label, kept, hint, refusal, children }: FieldProps) {
  return (
    <div className="flex flex-col gap-tight">
      <label htmlFor={name} className="text-sm text-muted-foreground">
        {label}
      </label>
      {kept === undefined ? null : (
        <p id={keptId(name)} className="text-sm">
          {kept}
        </p>
      )}
      {children}
      {hint ? (
        <p id={hintId(name)} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
      {refusal === undefined ? null : <Failure id={refusalId(name)}>{refusal}</Failure>}
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
        {...describedProps(name, { hint, refusal })}
        {...register(name, {
          setValueAs: type === 'number' ? readCount : readAs,
          validate: ruleFor(name),
        })}
      />
    </Field>
  )
}

// Kept is where a credential the file holds none of is read from instead:
// the keychain item for an address, a variable, or a command; the macOS
// keychain, perhaps, for a Slack user token's secrets, which workflow keeps
// there; the environment or the forge's CLI, perhaps, for the forge token,
// which the page cannot see; or nowhere.
export type Kept =
  | { from: 'keychain'; host: string }
  | { from: 'variable'; name: string }
  | { from: 'command' }
  | { from: 'macOS keychain' }
  | { from: 'forge sources' }
  | { from: 'nowhere' }

// keptWords say where a credential is read from, when the file holds none.
function keptWords(kept: Kept): string {
  switch (kept.from) {
    case 'keychain':
      return `Kept in your keychain, for ${kept.host}`
    case 'variable':
      return `Read from $${kept.name}`
    case 'command':
      return 'Printed by a command you set'
    case 'macOS keychain':
      return 'Not in the file; on macOS, your keychain may keep it'
    case 'forge sources':
      return "Not in the file; read from the environment or the forge's CLI, when either has one"
    case 'nowhere':
      return 'Not set'
  }
}

// useKeptWords say where a credential is kept: in the file, where the file
// holds it, or elsewhere — or, once one is typed, that the save keeps it in
// place of the stored one.
function useKeptWords(name: CredentialPath, label: string, elsewhere: Kept): string {
  const { holds } = useRemovals()
  const { dirtyFields } = useFormState<SettingsValues>({ name })
  const stored = holds({ credential: name })
  if (get(dirtyFields, name) === true) {
    const noun = label.charAt(0).toLowerCase() + label.slice(1)

    return stored ? `Replaces the stored ${noun} when you save` : 'Kept when you save'
  }

  return stored ? 'Stored in the file' : keptWords(elsewhere)
}

interface SecretFieldProps {
  register: Register
  name: CredentialPath
  label: string
  hint: string
  // elsewhere is where the credential is read from when the file holds none.
  elsewhere?: Kept
}

// SecretField is a credential: where it is kept, then the field it is typed
// into, which never shows it, and, beside it while the file holds one,
// Remove…, which takes it out of the file.
export function SecretField({
  register,
  name,
  label,
  hint,
  elsewhere = { from: 'nowhere' },
}: SecretFieldProps) {
  const outcome = useOutcome()
  const refusal = useRefusal(name)
  const kept = useKeptWords(name, label, elsewhere)

  return (
    <Field name={name} label={label} kept={kept} hint={hint} refusal={refusal}>
      <div className="flex flex-wrap items-center gap-item">
        <Input
          id={name}
          type="password"
          className="min-w-0 flex-1"
          {...describedProps(name, { kept, hint, refusal })}
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
      <Select id={name} {...describedProps(name, { hint, refusal })} {...register(name)}>
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

interface CredentialRemovalProps {
  removal: Removal
  // tell is the outcome line of whatever outlives this control: the field, or
  // the list, whose row a removal takes away.
  tell: Teller
}

// CredentialRemoval is Remove… beside a credential the file holds, which asks
// first — a removal writes the file at once and cannot be undone — and then
// writes the file as it was read without it, leaving the form's other edits
// as they are. What it did is said on the outcome line it is handed.
export function CredentialRemoval({ removal, tell }: CredentialRemovalProps) {
  const { holds, remove } = useRemovals()
  const [asking, setAsking] = useState(false)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const words = removalWords(removal)
  const run = useAsyncAction(() => remove(removal), {
    fallback: `${words} was not removed. Try again.`,
    done: () => `Removed ${words}.`,
    onStart: tell.clear,
    onDone: tell.say,
  })

  if (!holds(removal)) {
    return null
  }

  if (asking) {
    return (
      <RemoveQuestion
        removal={removal}
        onCancel={() => {
          setAsking(false)
          handBack()
        }}
        onRemove={() => {
          setAsking(false)
          handBack()
          void run.run()
        }}
      />
    )
  }

  return (
    <>
      <Button
        ref={opener}
        variant="secondary"
        size="sm"
        held={run.state === 'running'}
        aria-label={`Remove ${words}…`}
        onClick={() => {
          setAsking(true)
        }}
      >
        {run.state === 'running' ? 'Removing…' : 'Remove…'}
      </Button>
      {run.state === 'error' ? (
        <div className="basis-full">
          <Failure>{run.error}</Failure>
        </div>
      ) : null}
    </>
  )
}

interface RemoveQuestionProps {
  removal: Removal
  onCancel: () => void
  onRemove: () => void
}

// RemoveQuestion asks before a removal.
function RemoveQuestion({ removal, onCancel, onRemove }: RemoveQuestionProps) {
  return (
    <LastLook
      question={`Remove ${removalWords(removal)} from the file?`}
      cost={
        <>
          {removalConsequence(removal)}It cannot be undone: the file keeps no copy, and the save is
          made at once, without the form&apos;s other changes.
        </>
      }
      act="Remove"
      className="basis-full"
      onAct={onRemove}
      onCancel={onCancel}
    />
  )
}
