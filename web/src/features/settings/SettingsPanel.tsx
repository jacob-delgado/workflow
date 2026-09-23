import { useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Config } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { useConfig, useSaveConfig } from './configApi.ts'

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
  const { noun } = useForgeWords()
  const save = useSaveConfig()
  const [status, setStatus] = useState<SaveStatus>('idle')
  const [error, setError] = useState('')

  const onSubmit = handleSubmit(async (values) => {
    setStatus('saving')
    try {
      reset(await save(values))
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
            aria-describedby="jira.token-hint"
            className={inputClass}
            {...register('jira.token')}
          />
        </Field>
        <Field id="jira.user" label="User" hint="Empty authenticates with the token as a bearer.">
          <input
            id="jira.user"
            aria-describedby="jira.user-hint"
            className={inputClass}
            {...register('jira.user')}
          />
        </Field>
        <Field id="jira.project" label="Project">
          <input id="jira.project" className={inputClass} {...register('jira.project')} />
        </Field>
        <Field
          id="jira.review_status"
          label="Review status"
          hint={`The status an issue moves to once its ${noun} is open, e.g. "In Review". Empty makes no offer.`}
        >
          <input
            id="jira.review_status"
            className={inputClass}
            aria-describedby="jira.review_status-hint"
            {...register('jira.review_status')}
          />
        </Field>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" className="size-4" {...register('jira.markdown_comments')} />
          Write comments in Markdown, posted as Jira wiki markup
        </label>
      </Fieldset>

      <Fieldset legend="Messaging">
        <Field
          id="messaging.kind"
          label="Service"
          hint="Slack posts over a bot token or a webhook; the others post over a webhook."
        >
          <select
            id="messaging.kind"
            aria-describedby="messaging.kind-hint"
            className={inputClass}
            {...register('messaging.kind')}
          >
            <option value="slack">Slack</option>
            <option value="teams">Microsoft Teams</option>
            <option value="discord">Discord</option>
            <option value="webhook">Plain webhook</option>
          </select>
        </Field>
        <Field
          id="messaging.token"
          label="Bot token"
          hint="Slack only; leave as-is to keep the stored token."
        >
          <input
            id="messaging.token"
            type="password"
            aria-describedby="messaging.token-hint"
            className={inputClass}
            {...register('messaging.token')}
          />
        </Field>
        <Field
          id="messaging.webhook_url"
          label="Webhook URL"
          hint="A credential; leave as-is to keep it."
        >
          <input
            id="messaging.webhook_url"
            type="password"
            aria-describedby="messaging.webhook_url-hint"
            className={inputClass}
            {...register('messaging.webhook_url')}
          />
        </Field>
        <Field id="messaging.channel" label="Channel">
          <input id="messaging.channel" className={inputClass} {...register('messaging.channel')} />
        </Field>
        <Field id="messaging.announcement" label="Announcement">
          <input
            id="messaging.announcement"
            className={inputClass}
            {...register('messaging.announcement')}
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
            aria-describedby="forge.token-hint"
            className={inputClass}
            {...register('forge.token')}
          />
        </Field>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" className="size-4" {...register('forge.cli')} />
          Use the forge CLI for authentication
        </label>
      </Fieldset>

      <Fieldset legend="Commit">
        <Field
          id="commit.default_scope"
          label="Default scope"
          hint="Pre-fills the scope field, e.g. an area you scope commits to."
        >
          <input
            id="commit.default_scope"
            className={inputClass}
            aria-describedby="commit.default_scope-hint"
            {...register('commit.default_scope')}
          />
        </Field>
        <Field
          id="commit.types"
          label="Types"
          hint="Comma-separated, in the order to offer them; empty keeps the Conventional Commit types."
        >
          <input
            id="commit.types"
            className={inputClass}
            aria-describedby="commit.types-hint"
            {...register('commit.types', {
              setValueAs: (value: unknown) =>
                typeof value === 'string'
                  ? value
                      .split(',')
                      .map((type) => type.trim())
                      .filter(Boolean)
                  : value,
            })}
          />
        </Field>
        <Field
          id="commit.subject_limit"
          label="Subject limit"
          hint="The longest a subject may be, in characters; 0 keeps 72."
        >
          <input
            id="commit.subject_limit"
            type="number"
            min={0}
            className={inputClass}
            aria-describedby="commit.subject_limit-hint"
            {...register('commit.subject_limit', {
              setValueAs: (value: unknown) => (value === '' || value === null ? 0 : Number(value)),
            })}
          />
        </Field>
        <Field
          id="commit.refs_trailer"
          label="Issue trailer"
          hint='The trailer label added to a commit body; empty keeps "Refs".'
        >
          <input
            id="commit.refs_trailer"
            className={inputClass}
            aria-describedby="commit.refs_trailer-hint"
            {...register('commit.refs_trailer')}
          />
        </Field>
      </Fieldset>

      <Fieldset legend="Branch">
        <Field
          id="branch.template"
          label="Name template"
          hint="Uses {prefix}, {key} and {slug}; must contain {key}."
        >
          <input
            id="branch.template"
            className={inputClass}
            aria-describedby="branch.template-hint"
            {...register('branch.template')}
          />
        </Field>
        <Field
          id="branch.default_prefix"
          label="Default prefix"
          hint='The prefix for an unmapped type; empty keeps "feat".'
        >
          <input
            id="branch.default_prefix"
            className={inputClass}
            aria-describedby="branch.default_prefix-hint"
            {...register('branch.default_prefix')}
          />
        </Field>
        <Field
          id="branch.slug_limit"
          label="Slug limit"
          hint="Caps the summary slug's length; 0 keeps 48."
        >
          <input
            id="branch.slug_limit"
            type="number"
            min={0}
            className={inputClass}
            aria-describedby="branch.slug_limit-hint"
            {...register('branch.slug_limit', {
              setValueAs: (value: unknown) => (value === '' || value === null ? 0 : Number(value)),
            })}
          />
        </Field>
      </Fieldset>

      <Fieldset legend="Pull request">
        <Field
          id="pull_request.title_source"
          label="Title source"
          hint={`Where a ${noun}'s title comes from.`}
        >
          <select
            id="pull_request.title_source"
            className={inputClass}
            aria-describedby="pull_request.title_source-hint"
            {...register('pull_request.title_source')}
          >
            <option value="commit">The branch's oldest commit</option>
            <option value="issue">The issue it names</option>
          </select>
        </Field>
      </Fieldset>

      <Fieldset legend="Store">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" className="size-4" {...register('store.disabled')} />
          Keep nothing on disk between sessions
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

// The API throws an RFC 9457 problem details object as-is; surface its detail
// (or title) through the shared reader, else a fallback specific to saving.
export function errorMessage(caught: unknown): string {
  return apiErrorMessage(caught, 'The configuration could not be saved.')
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
      {hint ? (
        <p id={`${id}-hint`} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  )
}
