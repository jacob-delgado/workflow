import { useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import type { Config } from '@/api/generated/types.gen.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { saveConfig, useConfig } from './configApi.ts'

export function SettingsPanel() {
  const query = useConfig()

  if (query.isPending) {
    return <EmptyState>Loading the configuration…</EmptyState>
  }

  if (query.isError) {
    return <EmptyState>The configuration could not be loaded.</EmptyState>
  }

  return <ConfigForm config={query.data} />
}

type SaveStatus = 'idle' | 'saving' | 'saved' | 'error'

function ConfigForm({ config }: { config: Config }) {
  // The whole config seeds the form, so the sections and collections this form
  // does not edit (ui, timing, branch, commit, headers, views…) ride back
  // unchanged on save rather than being dropped.
  const { register, handleSubmit, reset } = useForm<Config>({ defaultValues: config })
  const [status, setStatus] = useState<SaveStatus>('idle')
  const [error, setError] = useState('')

  const onSubmit = handleSubmit(async (values) => {
    setStatus('saving')
    try {
      reset(await saveConfig(values))
      setError('')
      setStatus('saved')
    } catch (caught) {
      setError(errorMessage(caught))
      setStatus('error')
    }
  })

  return (
    <form
      onSubmit={(event) => {
        void onSubmit(event)
      }}
      className="mt-4 flex max-w-2xl flex-col gap-8"
    >
      <Fieldset legend="Jira">
        <Field id="jira.base_url" label="Base URL">
          <input
            id="jira.base_url"
            type="url"
            className={inputClass}
            {...register('jira.base_url')}
          />
        </Field>
        <Field id="jira.token" label="Token" hint="Leave as-is to keep the stored token.">
          <input
            id="jira.token"
            type="password"
            className={inputClass}
            {...register('jira.token')}
          />
        </Field>
        <Field id="jira.user" label="User" hint="Empty authenticates with the token as a bearer.">
          <input id="jira.user" className={inputClass} {...register('jira.user')} />
        </Field>
        <Field id="jira.project" label="Project">
          <input id="jira.project" className={inputClass} {...register('jira.project')} />
        </Field>
      </Fieldset>

      <Fieldset legend="Slack">
        <Field id="slack.token" label="Bot token" hint="Leave as-is to keep the stored token.">
          <input
            id="slack.token"
            type="password"
            className={inputClass}
            {...register('slack.token')}
          />
        </Field>
        <Field
          id="slack.webhook_url"
          label="Webhook URL"
          hint="A credential; leave as-is to keep it."
        >
          <input
            id="slack.webhook_url"
            type="password"
            className={inputClass}
            {...register('slack.webhook_url')}
          />
        </Field>
        <Field id="slack.channel" label="Channel">
          <input id="slack.channel" className={inputClass} {...register('slack.channel')} />
        </Field>
        <Field id="slack.announcement" label="Announcement">
          <input
            id="slack.announcement"
            className={inputClass}
            {...register('slack.announcement')}
          />
        </Field>
      </Fieldset>

      <Fieldset legend="Forge">
        <Field id="forge.kind" label="Kind">
          <select id="forge.kind" className={inputClass} {...register('forge.kind')}>
            <option value="">Auto-detect</option>
            <option value="github">GitHub</option>
            <option value="gitlab">GitLab</option>
          </select>
        </Field>
        <Field id="forge.host" label="Host">
          <input id="forge.host" className={inputClass} {...register('forge.host')} />
        </Field>
        <Field id="forge.token" label="Token" hint="Leave as-is to keep the stored token.">
          <input
            id="forge.token"
            type="password"
            className={inputClass}
            {...register('forge.token')}
          />
        </Field>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" className="size-4" {...register('forge.cli')} />
          Use the forge CLI for authentication
        </label>
      </Fieldset>

      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={status === 'saving'}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-60"
        >
          {status === 'saving' ? 'Saving…' : 'Save changes'}
        </button>
        <span role="status" className="text-sm">
          {status === 'saved' ? <span className="text-success">Saved.</span> : null}
          {status === 'error' ? <span className="text-destructive">{error}</span> : null}
        </span>
      </div>
    </form>
  )
}

// The API's error body is { code, message }, thrown as-is by the client rather
// than as an Error; surface its message when there is one, else a fallback.
export function errorMessage(caught: unknown): string {
  if (caught instanceof Error) {
    return caught.message
  }
  if (typeof caught === 'object' && caught !== null && 'message' in caught) {
    if (typeof caught.message === 'string') {
      return caught.message
    }
  }

  return 'The configuration could not be saved.'
}

const inputClass =
  'rounded-md border border-input bg-transparent px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'

function Fieldset({ legend, children }: { legend: string; children: ReactNode }) {
  return (
    <fieldset className="flex flex-col gap-4">
      <legend className="text-sm font-semibold text-muted-foreground uppercase">{legend}</legend>
      {children}
    </fieldset>
  )
}

function Field({
  id,
  label,
  hint,
  children,
}: {
  id: string
  label: string
  hint?: string
  children: ReactNode
}) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm text-muted-foreground">
        {label}
      </label>
      {children}
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  )
}
