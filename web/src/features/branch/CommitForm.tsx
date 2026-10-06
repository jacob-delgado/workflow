import { useCallback, useEffect, useRef } from 'react'
import { useForm, type UseFormRegister } from 'react-hook-form'
import type { Branch } from '@/api/generated/types.gen.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { Input, Select, TextArea } from '@/lib/Field.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn } from '@/lib/utils.ts'
import { commitChanges } from './commitApi.ts'

interface CommitFields {
  type: string
  scope: string
  subject: string
  body: string
  breaking: boolean
}

interface CommitFormProps {
  canCommit: boolean
  // waiting is what the form waits for, said at its foot, when it says so there.
  waiting?: string
  suggestedScope: string
  commitTypes: string[]
}

// CommitForm commits the staged changes with a Conventional Commit message. The
// server assembles the message and adds the Refs trailer for the branch's issue;
// it says which commit it made, and a refusal is shown inline.
// It stays in place while nothing is staged, its button off — and waiting, when
// given, says at its foot what it waits for — so a message can be written
// before the files are. It
// offers the commit types the server allows, in its order, and opens on the
// scope the server suggests, the terminal composer's own.
export function CommitForm({ canCommit, waiting, suggestedScope, commitTypes }: CommitFormProps) {
  const { register, handleSubmit, reset, getValues, setValue } = useForm<CommitFields>({
    defaultValues: { type: 'fix', scope: suggestedScope, subject: '', body: '', breaking: false },
  })
  const applyScope = useCallback(
    (scope: string) => {
      setValue('scope', scope)
    },
    [setValue],
  )
  const scopeSuggestion = useScopeSuggestion(suggestedScope, applyScope)
  const outcome = useOutcome()

  // Keep the chosen type one the convention allows, so a team whose types
  // change after the form opens, or exclude "fix", does not submit a type the
  // server rejects.
  useEffect(() => {
    if (!commitTypes.includes(getValues('type'))) {
      setValue('type', commitTypes[0] ?? 'fix')
    }
  }, [commitTypes, getValues, setValue])

  const commit = useAsyncAction(
    async (fields: CommitFields) => {
      const committed = await commitChanges({
        type: fields.type,
        subject: fields.subject,
        scope: fields.scope,
        body: fields.body,
        breaking: fields.breaking,
      })
      reset(nextMessage(fields, commitTypes, suggestedScope))
      scopeSuggestion.release()

      return committed
    },
    {
      fallback: 'Nothing was committed. Try again, or commit from a terminal to see why.',
      done: (committed, fields) => `Committed ${headline(committed, fields)}.`,
      onStart: outcome.clear,
      onDone: outcome.say,
    },
  )
  const onSubmit = handleSubmit((fields) => commit.run(fields))

  return (
    <form
      aria-label="Commit staged changes"
      onSubmit={(event) => {
        void onSubmit(event)
      }}
      className="flex flex-col gap-group rounded-lg border border-border p-4"
    >
      <MessageFields
        register={register}
        commitTypes={commitTypes}
        onScopeTyping={scopeSuggestion.typing}
      />
      <div className="flex items-center gap-item">
        <Button
          variant="primary"
          type="submit"
          disabled={!canCommit}
          held={commit.state === 'running'}
          className="self-start"
        >
          {commit.state === 'running' ? 'Committing…' : 'Commit staged changes'}
        </Button>
        {waiting === undefined ? null : <p className="text-sm text-muted-foreground">{waiting}</p>}
        <OutcomeLine said={outcome.said} />
        {commit.state === 'error' ? (
          <p role="alert" className="text-sm whitespace-pre-line text-destructive">
            {commit.error}
          </p>
        ) : null}
      </div>
    </form>
  )
}

// nextMessage is what the form holds once a commit lands: a type the
// convention allows, not the static "fix" default, which a team that excludes
// it would leave selected and the server reject; and the scope just used,
// which a store that keeps it suggests from now on, or the suggestion a blank
// scope left standing.
function nextMessage(used: CommitFields, types: string[], suggested: string): CommitFields {
  const scope = used.scope.trim()

  return {
    type: types[0] ?? 'fix',
    scope: scope === '' ? suggested : scope,
    subject: '',
    body: '',
    breaking: false,
  }
}

interface MessageFieldsProps {
  register: UseFormRegister<CommitFields>
  commitTypes: string[]
  // onScopeTyping marks the scope as the user's, so no suggestion replaces it.
  onScopeTyping: () => void
}

// MessageFields are the parts of a Conventional Commit message: its type and
// scope, its subject and body, and whether it breaks anything.
// The commit's c puts the focus in its first field, Type, as the terminal's c
// opens the composer there.
function MessageFields({ register, commitTypes, onScopeTyping }: MessageFieldsProps) {
  const typeField = register('type')
  const typeSelect = useRef<HTMLSelectElement>(null)
  const shortcut = useShortcut('commit', typeSelect, 'focus')

  return (
    <>
      <div className="flex gap-item">
        <label className={labelClass}>
          Type
          <Select
            {...typeField}
            ref={(select) => {
              typeField.ref(select)
              typeSelect.current = select
            }}
            aria-keyshortcuts={shortcut}
          >
            {commitTypes.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </Select>
        </label>
        <label className={cn('min-w-0 flex-1', labelClass)}>
          Scope (optional)
          <Input {...register('scope', { onChange: onScopeTyping })} />
        </label>
      </div>

      <label className={labelClass}>
        Subject
        <Input
          {...register('subject')}
          required
          placeholder="what the change does, in the imperative"
        />
      </label>

      <label className={labelClass}>
        Body (optional)
        <TextArea {...register('body')} rows={3} />
      </label>

      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" {...register('breaking')} className="size-4" />
        Breaking change
      </label>
    </>
  )
}

// headline names a commit the branch now carries: its short hash and the
// subject git recorded — the newest commit, when it is the one on HEAD — or,
// when the branch's list stops short of HEAD, the header the fields make. A
// branch with no head is one the server could not read back after the commit,
// so there is no hash to name and the header stands alone.
function headline(branch: Branch, fields: CommitFields): string {
  if (branch.head === '') {
    return header(fields)
  }

  const newest = branch.commits.at(-1)
  const subject =
    newest !== undefined && branch.head.startsWith(newest.hash) ? newest.subject : header(fields)

  return `${branch.head.slice(0, 7)} ${subject}`
}

// header is the Conventional Commit header the fields make, as the server
// assembles it: the type, the scope in parentheses when there is one, a ! for a
// breaking change, then the subject.
function header(fields: CommitFields): string {
  const scope = fields.scope.trim()
  const scoped = scope === '' ? fields.type : `${fields.type}(${scope})`

  return `${scoped}${fields.breaking ? '!' : ''}: ${fields.subject.trim()}`
}

// useScopeSuggestion keeps the scope field on the scope the stream suggests
// while no one has typed in it: a later frame's suggestion — a default_scope
// saved in Settings, say — applies to an untouched field, and never to one
// someone is typing in. typing marks the field as someone's; release hands it
// back, once the commit it was typed for has landed.
function useScopeSuggestion(suggested: string, apply: (scope: string) => void) {
  const typed = useRef(false)

  useEffect(() => {
    if (!typed.current) {
      apply(suggested)
    }
  }, [suggested, apply])

  return {
    typing: () => {
      typed.current = true
    },
    release: () => {
      typed.current = false
    },
  }
}

const labelClass = 'flex flex-col gap-tight text-sm text-muted-foreground'
