import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { apiErrorMessage } from '@/api/apiError.ts'
import { useConfig } from '@/features/settings/configApi.ts'
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

type CommitStatus = 'idle' | 'committing' | 'error'

// CommitForm commits the staged changes with a Conventional Commit message. The
// server assembles the message and adds the Refs trailer for the branch's issue;
// on success the event stream reflects the commit, and a refusal is shown inline.
// It stays in place while nothing is staged — blocked says why, in the form,
// with its button off — so a message can be written before the files are.
export function CommitForm({ blocked }: { blocked: string | null }) {
  const { data: config } = useConfig()
  // A team's own commit types, in the order to offer them, or the built-in set.
  const commitTypes = useMemo(() => {
    const configured = config?.commit.types
    return configured && configured.length > 0 ? configured : defaultCommitTypes
  }, [config])

  const { register, handleSubmit, reset, getValues, setValue } = useForm<CommitFields>({
    defaultValues: { type: 'fix', scope: '', subject: '', body: '', breaking: false },
  })
  const [status, setStatus] = useState<CommitStatus>('idle')
  const [error, setError] = useState('')

  // Keep the chosen type one the convention allows, so a team whose types load
  // after the form, or exclude "fix", does not submit a type the server rejects.
  useEffect(() => {
    if (!commitTypes.includes(getValues('type'))) {
      setValue('type', commitTypes[0] ?? 'fix')
    }
  }, [commitTypes, getValues, setValue])

  const onSubmit = handleSubmit(async (fields) => {
    setStatus('committing')
    try {
      await commitChanges({
        type: fields.type,
        subject: fields.subject,
        scope: fields.scope,
        body: fields.body,
        breaking: fields.breaking,
      })
      // Reset to a type the convention allows, not the static "fix" default,
      // which a team that excludes it would leave selected and the server reject.
      reset({ type: commitTypes[0] ?? 'fix', scope: '', subject: '', body: '', breaking: false })
      setError('')
      setStatus('idle')
    } catch (caught) {
      setError(apiErrorMessage(caught, 'The commit could not be created.'))
      setStatus('error')
    }
  })

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
          <input {...register('scope')} className={commitInputClass} />
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
          disabled={blocked !== null || status === 'committing'}
          className="self-start rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
        >
          {status === 'committing' ? 'Committing…' : 'Commit staged changes'}
        </button>
        {blocked === null ? null : <p className="text-sm text-muted-foreground">{blocked}</p>}
        {status === 'error' ? (
          <p role="alert" className="text-sm whitespace-pre-line text-destructive">
            {error}
          </p>
        ) : null}
      </div>
    </form>
  )
}

const commitInputClass =
  'rounded-md border border-input bg-transparent px-3 py-2 text-sm text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
