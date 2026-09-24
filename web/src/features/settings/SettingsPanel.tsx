import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import type { Config } from '@/api/generated/types.gen.ts'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { useConfig, useSaveConfig } from './configApi.ts'
import { BranchFieldset } from './fieldsets/BranchFieldset.tsx'
import { CommitFieldset } from './fieldsets/CommitFieldset.tsx'
import { ForgeFieldset } from './fieldsets/ForgeFieldset.tsx'
import { JiraFieldset } from './fieldsets/JiraFieldset.tsx'
import { MessagingFieldset } from './fieldsets/MessagingFieldset.tsx'
import { PullRequestFieldset, StoreFieldset } from './fieldsets/PullRequestAndStoreFieldsets.tsx'

export function SettingsPanel() {
  const query = useConfig()
  // A Retry is swapped for the form it loads, so the form takes the focus the
  // Retry had rather than letting it fall to the page.
  const [retried, setRetried] = useState(false)

  if (query.isPending) {
    return <EmptyState>Loading the configuration…</EmptyState>
  }

  if (query.isError) {
    return (
      <EmptyState>
        <span className="flex flex-col items-center gap-group">
          The configuration could not be loaded.
          <button
            type="button"
            disabled={query.isFetching}
            onClick={() => {
              setRetried(true)
              void query.refetch()
            }}
            className="rounded-md border border-input px-3 py-1.5 text-foreground hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
          >
            {query.isFetching ? 'Retrying…' : 'Retry'}
          </button>
        </span>
      </EmptyState>
    )
  }

  return <ConfigForm config={query.data} takesFocus={retried} />
}

// ConfigForm edits the configuration, one fieldset per section it can edit. It
// takes focus, on its first field, only when it replaces a control that had
// it — a Retry — and otherwise leaves focus where the section change put it.
function ConfigForm({ config, takesFocus }: { config: Config; takesFocus: boolean }) {
  // The whole config seeds the form, so the sections and collections this form
  // does not edit (ui, timing, headers, views…) ride back unchanged on save
  // rather than being dropped.
  const { register, handleSubmit, reset, setFocus } = useForm<Config>({ defaultValues: config })
  const saveConfig = useSaveConfig()
  const save = useAsyncAction(
    async (values: Config) => {
      reset(await saveConfig(values))
    },
    { fallback: 'The configuration was not saved. Try again — your edits are still in the form.' },
  )
  const onSubmit = handleSubmit((values) => save.run(values))

  useEffect(() => {
    if (takesFocus) {
      setFocus('jira.base_url')
    }
  }, [takesFocus, setFocus])

  return (
    <form
      onSubmit={(event) => {
        void onSubmit(event)
      }}
      className="flex max-w-2xl flex-col gap-section"
    >
      <JiraFieldset register={register} />
      <MessagingFieldset register={register} />
      <ForgeFieldset register={register} />
      <CommitFieldset register={register} />
      <BranchFieldset register={register} />
      <PullRequestFieldset register={register} />
      <StoreFieldset register={register} />

      <div className="flex items-center gap-item">
        <button
          type="submit"
          disabled={save.state === 'running'}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
        >
          {save.state === 'running' ? 'Saving…' : 'Save changes'}
        </button>
        <span role="status" className="text-sm">
          {save.state === 'done' ? <span className="text-success">Saved.</span> : null}
          {save.state === 'error' ? <span className="text-destructive">{save.error}</span> : null}
        </span>
      </div>
    </form>
  )
}
