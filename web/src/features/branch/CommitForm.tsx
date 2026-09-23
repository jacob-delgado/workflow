import { useCallback, useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import type { Branch } from '@/api/generated/types.gen.ts'
import { useConfig } from '@/features/settings/configApi.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { commitChanges } from './commitApi.ts'

// The built-in Conventional Commit types, in the order the terminal composer
// offers them, used when a team configures none of its own.
const defaultCommitTypes = [
  'feat',
  'fix',
  'docs',
  'refactor',
  'test',
  'perf',
  'build',
  'ci',
  'chore',
  'style',
  'revert',
]

interface CommitFields {
  type: string
  scope: string
  subject: string
  body: string
  breaking: boolean
}

// CommitForm commits the staged changes with a Conventional Commit message. The
// server assembles the message and adds the Refs trailer for the branch's issue;
// it says which commit it made, and a refusal is shown inline.
// It stays in place while nothing is staged — blocked says why, in the form,
// with its button off — so a message can be written before the files are. It
// opens on the scope the server suggests, the terminal composer's own.
export function CommitForm({
  blocked,
  suggestedScope,
}: {
  blocked: string | null
  suggestedScope: string
}) {
  const { data: config } = useConfig()
  // A team's own commit types, in the order to offer them, or the built-in set.
  const commitTypes = useMemo(() => {
    const configured = config?.commit.types
    return configured && configured.length > 0 ? configured : defaultCommitTypes
  }, [config])

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

  // Keep the chosen type one the convention allows, so a team whose types load
  // after the form, or exclude "fix", does not submit a type the server rejects.
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
      // Reset to a type the convention allows, not the static "fix" default,
      // which a team that excludes it would leave selected and the server reject;
      // and to the scope just used, which a store that keeps it suggests from
      // now on, or to the suggestion a blank scope left standing.
      const usedScope = fields.scope.trim()
      reset({
        type: commitTypes[0] ?? 'fix',
        scope: usedScope === '' ? suggestedScope : usedScope,
        subject: '',
        body: '',
        breaking: false,
      })
      scopeSuggestion.release()

      return committed
    },
    {
      fallback: 'The commit could not be created.',
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
      className="flex flex-col gap-3 rounded-md border border-border p-4"
    >
      <div className="flex gap-2">
        <label className="flex flex-col gap-1 text-xs text-muted-foreground">
          Type
          <select {...register('type')} className={commitInputClass}>
            {commitTypes.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-1 flex-col gap-1 text-xs text-muted-foreground">
          Scope (optional)
          <input
            {...register('scope', { onChange: scopeSuggestion.typing })}
            className={commitInputClass}
          />
        </label>
      </div>

      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        Subject
        <input
          {...register('subject')}
          required
          placeholder="what the change does, in the imperative"
          className={commitInputClass}
        />
      </label>

      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        Body (optional)
        <textarea {...register('body')} rows={3} className={commitInputClass} />
      </label>

      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" {...register('breaking')} className="size-4" />
        Breaking change
      </label>

      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={blocked !== null || commit.state === 'running'}
          className="self-start rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
        >
          {commit.state === 'running' ? 'Committing…' : 'Commit staged changes'}
        </button>
        {blocked === null ? null : <p className="text-sm text-muted-foreground">{blocked}</p>}
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

// headline names a commit the branch now carries: its short hash and the
// subject git recorded — the newest commit, when it is the one on HEAD — or,
// when the branch's list stops short of HEAD, the header the fields make.
function headline(branch: Branch, fields: CommitFields): string {
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

const commitInputClass =
  'rounded-md border border-input bg-transparent px-3 py-2 text-sm text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
