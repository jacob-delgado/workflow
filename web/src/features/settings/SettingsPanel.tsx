import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { apiErrorMessage, problemCode } from '@/api/apiError.ts'
import type { Config, SetupResult } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { Failure, Reading, Unread } from '@/lib/Status.tsx'
import { type AsyncState, useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, contentMeasure } from '@/lib/utils.ts'
import { sectionHeadingId } from '@/shell/sections.ts'
import {
  changedSinceRead,
  type ConfigRead,
  useConfigRead,
  useReloadConfig,
  useSaveConfig,
} from './configApi.ts'
import { BranchFieldset } from './fieldsets/BranchFieldset.tsx'
import { CommitFieldset } from './fieldsets/CommitFieldset.tsx'
import { ForgeFieldset } from './fieldsets/ForgeFieldset.tsx'
import { JiraFieldset } from './fieldsets/JiraFieldset.tsx'
import { KeyboardFieldset } from './fieldsets/KeyboardFieldset.tsx'
import { MessagingFieldset } from './fieldsets/MessagingFieldset.tsx'
import { PullRequestFieldset, StoreFieldset } from './fieldsets/PullRequestAndStoreFieldsets.tsx'
import { TaskwarriorFieldset } from './fieldsets/TaskwarriorFieldset.tsx'
import { LocalData } from './people/LocalData.tsx'
import { PeopleAndGroups } from './people/PeopleAndGroups.tsx'
import { SetupArea } from './SetupForm.tsx'

// SettingsPanel is the configuration form, and below it the areas that save on
// their own rather than with the file: people and groups, and the local data.
export function SettingsPanel() {
  return (
    <div className={cn('flex flex-col gap-section', contentMeasure)}>
      <ConfigArea />
      <PeopleAndGroups />
      <LocalData />
    </div>
  )
}

// ConfigArea is the configuration form, once the file is read, or why it is
// not — or, where no file applies, the setup that writes a first one, and once
// it has, what it wrote above the form.
function ConfigArea() {
  const query = useConfigRead()
  // A Try again is swapped for the form it loads, so the form takes the focus the
  // Try again had rather than letting it fall to the page.
  const [retried, setRetried] = useState(false)
  const [setUp, setSetUp] = useState<SetupResult | null>(null)

  if (query.isError && problemCode(query.error) === 'not_found') {
    return <SetupArea onWritten={setSetUp} />
  }

  // Settings opens on a fresh read rather than on the cached one: a form seeded
  // from a read the file has moved on from would only learn so on its save.
  if (query.isPending || (query.isFetching && !query.isFetchedAfterMount)) {
    return <Reading>Reading the configuration…</Reading>
  }

  if (query.isError) {
    return (
      <Unread
        reason={apiErrorMessage(query.error, 'The configuration could not be read.')}
        refusals={query.errorUpdateCount}
        retrying={query.isFetching}
        onRetry={() => {
          setRetried(true)
          void query.refetch()
        }}
      />
    )
  }

  return (
    <>
      {setUp === null ? null : <SetUpSaid result={setUp} />}
      <ConfigForm read={query.data} takesFocus={retried} />
    </>
  )
}

// SetUpSaid says what a setup wrote, and what is left to do about it. It takes
// the focus the setup's write had, which went with the setup.
function SetUpSaid({ result }: { result: SetupResult }) {
  const said = useFocusOnMount<HTMLParagraphElement>()
  const parts = [`Wrote ${result.shown}.`]
  if (result.jira_user !== '') {
    parts.push(`Jira knows the token as ${result.jira_user}.`)
  }
  if (result.keychain) {
    parts.push('Your keychain keeps the token.')
  }
  if (result.not_ignored) {
    parts.push('git does not ignore it, and it holds credentials: add it to .gitignore.')
  }
  if (!result.reopened) {
    parts.push('Restart workflow --web to work with it.')
  }

  return (
    <p ref={said} role="status" tabIndex={-1} className="text-sm text-success">
      {parts.join(' ')}
    </p>
  )
}

// ConfigForm edits the configuration, one fieldset per section it can edit. It
// takes focus, on its first field, only when it replaces a control that had
// it — a Try again, or a Reload — and otherwise leaves focus where the section
// change put it.
function ConfigForm({ read, takesFocus }: { read: ConfigRead; takesFocus: boolean }) {
  // The whole config seeds the form, so every key no fieldset registers — ui,
  // timing, jira.views and the token commands among them — rides back unchanged
  // on save rather than being dropped.
  const { register, handleSubmit, reset, setFocus, control, formState } = useForm<Config>({
    defaultValues: read.config,
  })
  // The revision of the file the form's values stand for: the read that seeded
  // it, then each save and each reload. A save names it, so the server refuses
  // it over a change the form has not seen, even once the cached read has moved
  // on — unless that change lands between the server's check and its write.
  const [revision, setRevision] = useState(read.revision)
  // Whether the last save was refused because the file changed since the form
  // read it: the refusal that Reload, not another save, answers.
  const [changed, setChanged] = useState(false)
  // How many times the form's first field has been asked to take focus: once
  // when the form replaces a Try again, and after each Reload, which takes its own
  // button away. The field is focused once the form has drawn it.
  const [focusRequests, setFocusRequests] = useState(takesFocus ? 1 : 0)
  const saveConfig = useSaveConfig()
  const reloadConfig = useReloadConfig()
  const outcome = useOutcome()
  const seed = (next: ConfigRead) => {
    reset(next.config)
    setRevision(next.revision)
  }
  const save = useAsyncAction(
    async (values: Config) => {
      try {
        seed(await saveConfig(values, revision))
        setChanged(false)
      } catch (caught) {
        setChanged(changedSinceRead(caught))
        throw caught
      }
    },
    {
      fallback: 'The configuration was not saved. Try again — your edits are still in the form.',
      done: () => 'Saved.',
      onStart: outcome.clear,
      onDone: outcome.say,
    },
  )
  // Reload swaps the edits for the file as it is now and clears the refusal,
  // taking the Reload button away, so the focus it had goes to the form.
  const reload = useAsyncAction(
    async () => {
      seed(await reloadConfig())
      setChanged(false)
      save.reset()
      setFocusRequests((requests) => requests + 1)
    },
    { fallback: 'The configuration could not be read. Try Reload again.' },
  )
  const onSubmit = handleSubmit((values) => save.run(values))

  useEffect(() => {
    if (focusRequests > 0) {
      setFocus('jira.base_url')
    }
  }, [focusRequests, setFocus])

  return (
    <form
      aria-labelledby={sectionHeadingId}
      onSubmit={(event) => {
        void onSubmit(event)
      }}
      className="flex flex-col gap-section"
    >
      <JiraFieldset
        register={register}
        control={control}
        storedToken={formState.defaultValues?.jira?.token ?? null}
      />
      <MessagingFieldset register={register} />
      <ForgeFieldset register={register} />
      <CommitFieldset register={register} />
      <BranchFieldset register={register} />
      <PullRequestFieldset register={register} />
      <StoreFieldset register={register} />
      <TaskwarriorFieldset register={register} />
      <KeyboardFieldset register={register} />

      {/* A refusal ChangedSinceRead explains is not said a second time. */}
      <SaveControls
        saving={save.state === 'running'}
        said={outcome.said}
        error={changed || save.state !== 'error' ? '' : save.error}
      />
      {changed ? <ChangedSinceRead reload={reload} /> : null}
    </form>
  )
}

interface SaveControlsProps {
  saving: boolean
  said: Parameters<typeof OutcomeLine>[0]['said']
  error: string
}

// SaveControls is the Save button and what the last save said: that it saved,
// on the form's outcome line, or why it did not, as a failure — unless the file
// changed since the form read it, which ChangedSinceRead says instead.
function SaveControls({ saving, said, error }: SaveControlsProps) {
  return (
    <div className="flex flex-col items-start gap-item">
      <div className="flex items-center gap-item">
        <Button variant="primary" type="submit" held={saving}>
          {saving ? 'Saving…' : 'Save changes'}
        </Button>
        <OutcomeLine said={said} />
      </div>
      {error === '' ? null : <Failure>{error}</Failure>}
    </div>
  )
}

// ReloadAction is the one-shot read Reload runs.
interface ReloadAction {
  state: AsyncState
  error: string
  run: () => Promise<void>
}

// ChangedSinceRead says a save was refused because the file changed since the
// form read it, what Reload will do to the edits, and offers it.
function ChangedSinceRead({ reload }: { reload: ReloadAction }) {
  return (
    <div className="flex flex-col items-start gap-item">
      <p role="alert" className="text-sm text-destructive">
        The configuration file changed after Settings read it — edited on disk, or saved from
        another tab — so nothing was saved. Reload reads it again in place of your edits here; then
        make your change again.
      </p>
      <Button
        variant="secondary"
        held={reload.state === 'running'}
        onClick={() => {
          void reload.run()
        }}
      >
        {reload.state === 'running' ? 'Reloading…' : 'Reload'}
      </Button>
      {reload.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {reload.error}
        </p>
      ) : null}
    </div>
  )
}
