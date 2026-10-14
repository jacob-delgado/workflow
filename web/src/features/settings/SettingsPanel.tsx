import { type ReactNode, useEffect, useState } from 'react'
import { type Control, type UseFormRegister, type UseFormReset, useForm } from 'react-hook-form'
import { apiErrorMessage, problemCode } from '@/api/apiError.ts'
import type { Config, SetupResult } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { resultGround, useOutcome } from '@/lib/Outcome.tsx'
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
import { StoreFieldset, TimingFieldset } from './fieldsets/AdvancedFieldsets.tsx'
import { BranchFieldset, CommitFieldset } from './fieldsets/CommitAndBranchFieldsets.tsx'
import { ForgeFieldset, PullRequestFieldset } from './fieldsets/ForgeAndPullRequestFieldsets.tsx'
import { JiraFieldset } from './fieldsets/JiraFieldset.tsx'
import { MessagingFieldset } from './fieldsets/MessagingFieldset.tsx'
import { TaskwarriorFieldset } from './fieldsets/TaskwarriorFieldset.tsx'
import { KeyboardFieldset, TerminalFieldset } from './fieldsets/TerminalAndKeyFieldsets.tsx'
import { LocalData } from './people/LocalData.tsx'
import { PeopleAndGroups } from './people/PeopleAndGroups.tsx'
import { changedKeys, configOf, formValues, type SettingsValues } from './formValues.ts'
import { type Removal, RemovalsContext, stored, without } from './removal.ts'
import { SaveBar } from './SaveBar.tsx'
import { ConfigPanel, SettingsSections, unsavedBySection } from './SettingsSections.tsx'
import { SetupArea } from './SetupForm.tsx'

// SettingsPanel is the configuration, a section at a time beside the list of
// them, with the areas that save on their own rather than with the file —
// people and groups, and the local data — at the list's foot. Until the file
// is read, it is why it is not, or the setup that writes a first one, above
// those two areas.
export function SettingsPanel() {
  return <ConfigArea />
}

// Unformed is Settings without the configuration form: what stands in for it,
// above the areas that save on their own.
function Unformed({ children }: { children: ReactNode }) {
  return (
    <div className={cn('flex flex-col gap-section', contentMeasure)}>
      {children}
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
    return (
      <Unformed>
        <SetupArea onWritten={setSetUp} />
      </Unformed>
    )
  }

  // Settings opens on a fresh read rather than on the cached one: a form seeded
  // from a read the file has moved on from would only learn so on its save.
  if (query.isPending || (query.isFetching && !query.isFetchedAfterMount)) {
    return (
      <Unformed>
        <Reading>Reading the configuration…</Reading>
      </Unformed>
    )
  }

  if (query.isError) {
    return (
      <Unformed>
        <Unread
          reason={apiErrorMessage(query.error, 'The configuration could not be read.')}
          refusals={query.errorUpdateCount}
          retrying={query.isFetching}
          onRetry={() => {
            setRetried(true)
            void query.refetch()
          }}
        />
      </Unformed>
    )
  }

  return (
    <div className="flex flex-col gap-section">
      {setUp === null ? null : <SetUpSaid result={setUp} />}
      <ConfigForm read={query.data} takesFocus={retried} />
    </div>
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
    <p ref={said} role="status" tabIndex={-1} className={resultGround}>
      {parts.join(' ')}
    </p>
  )
}

// ConfigForm edits the configuration, one fieldset per section it can edit. It
// takes focus, on its first field, only when it replaces a control that had
// it — a Try again, or a Reload — and otherwise leaves focus where the section
// change put it.
function ConfigForm({ read, takesFocus }: { read: ConfigRead; takesFocus: boolean }) {
  // The whole config seeds the form, so every key no fieldset registers — the
  // token commands, the Slack access token and its expiry — rides back
  // unchanged on save rather than being dropped.
  const { register, handleSubmit, reset, setFocus, control, formState, watch } =
    useForm<SettingsValues>({ defaultValues: formValues(read.config) })
  const file = useFormFile(read, reset)
  const changed = changedKeys(configOf(watch()), configOf(formValues(file.seeded)))
  // How many times the form's first field has been asked to take focus: once
  // when the form replaces a Try again, and after each Reload, which takes its own
  // button away. The field is focused once the form has drawn it.
  const [focusRequests, setFocusRequests] = useState(takesFocus ? 1 : 0)
  const reloadConfig = useReloadConfig()
  const outcome = useOutcome()
  const [shown, setShown] = useState('jira')
  const save = useAsyncAction(
    (values: SettingsValues) => file.write(configOf(values), { keepEdits: false }),
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
      file.seed(await reloadConfig(), { keepEdits: false })
      file.setChanged(false)
      save.reset()
      setShown('jira')
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
    <RemovalsContext value={file.removals}>
      <SettingsSections shown={shown} onShow={setShown} unsaved={unsavedBySection(changed)}>
        <form
          aria-labelledby={sectionHeadingId}
          onSubmit={(event) => {
            void onSubmit(event)
          }}
          className="flex flex-col gap-section"
        >
          <ConfigPanels
            register={register}
            control={control}
            storedToken={formState.defaultValues?.jira?.token ?? null}
          />

          {/* A refusal ChangedSinceRead explains is not said a second time. */}
          <SaveBar
            unsaved={changed.length}
            saving={save.state === 'running'}
            said={outcome.said}
            error={file.changed || save.state !== 'error' ? '' : save.error}
            onDiscard={() => {
              outcome.clear()
              file.discard()
            }}
          >
            {file.changed ? <ChangedSinceRead reload={reload} /> : null}
          </SaveBar>
        </form>
      </SettingsSections>
    </RemovalsContext>
  )
}

interface ConfigPanelsProps {
  register: UseFormRegister<SettingsValues>
  control: Control<SettingsValues>
  storedToken: string | null
}

// ConfigPanels are the configuration's sections, each its fieldsets.
function ConfigPanels({ register, control, storedToken }: ConfigPanelsProps) {
  return (
    <>
      <ConfigPanel id="jira">
        <JiraFieldset register={register} control={control} storedToken={storedToken} />
      </ConfigPanel>
      <ConfigPanel id="messaging">
        <MessagingFieldset register={register} control={control} />
      </ConfigPanel>
      <ConfigPanel id="forge">
        <ForgeFieldset register={register} />
        <PullRequestFieldset register={register} />
      </ConfigPanel>
      <ConfigPanel id="commits">
        <CommitFieldset register={register} />
        <BranchFieldset register={register} control={control} />
      </ConfigPanel>
      <ConfigPanel id="taskwarrior">
        <TaskwarriorFieldset register={register} />
      </ConfigPanel>
      <ConfigPanel id="terminal">
        <TerminalFieldset register={register} />
        <KeyboardFieldset register={register} />
      </ConfigPanel>
      <ConfigPanel id="advanced">
        <TimingFieldset register={register} />
        <StoreFieldset register={register} />
      </ConfigPanel>
    </>
  )
}

// Seeding is how a read or a write is taken up: in place of the form's edits,
// or under them, leaving them as they are.
interface Seeding {
  keepEdits: boolean
}

// useFormFile is the form's hold on the file it edits: the revision its
// values stand for — the read that seeded it, then each write and each reload,
// which a write names so the server refuses it over a change the form has not
// seen, unless that change lands between the server's check and its write —
// the configuration as last read or written, and whether the last write was
// refused because the file changed since: the refusal Reload, not another
// write, answers. A removal writes the file as read without the credential,
// and takes up what it wrote under the form's edits.
function useFormFile(read: ConfigRead, reset: UseFormReset<SettingsValues>) {
  const [revision, setRevision] = useState(read.revision)
  const [seeded, setSeeded] = useState(read.config)
  const [changed, setChanged] = useState(false)
  const saveConfig = useSaveConfig()
  const seed = (next: ConfigRead, { keepEdits }: Seeding) => {
    reset(formValues(next.config), { keepDirtyValues: keepEdits })
    setSeeded(next.config)
    setRevision(next.revision)
  }
  const write = async (config: Config, seeding: Seeding) => {
    try {
      seed(await saveConfig(config, revision), seeding)
      setChanged(false)
    } catch (caught) {
      setChanged(changedSinceRead(caught))
      throw caught
    }
  }

  return {
    changed,
    setChanged,
    seed,
    write,
    seeded,
    discard: () => {
      reset(formValues(seeded))
    },
    removals: {
      holds: (removal: Removal) => stored(seeded, removal),
      remove: (removal: Removal) => write(without(seeded, removal), { keepEdits: true }),
    },
  }
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
      <Failure>
        The configuration file changed after Settings read it — edited on disk, or saved from
        another tab — so nothing was saved. Reload reads it again in place of your edits here; then
        make your change again.
      </Failure>
      <Button
        variant="secondary"
        held={reload.state === 'running'}
        onClick={() => {
          void reload.run()
        }}
      >
        {reload.state === 'running' ? 'Reloading…' : 'Reload'}
      </Button>
      {reload.state === 'error' ? <Failure>{reload.error}</Failure> : null}
    </div>
  )
}
