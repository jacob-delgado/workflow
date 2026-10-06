import { useId } from 'react'
import { type Control, type UseFormRegister, useForm, useWatch } from 'react-hook-form'
import { apiErrorMessage } from '@/api/apiError.ts'
import type {
  SetupOffer,
  SetupPlace,
  SetupPlaceName,
  SetupRequest,
  SetupResult,
} from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { Button } from '@/lib/Button.tsx'
import { Input } from '@/lib/Field.tsx'
import { Failure, Reading, Unread } from '@/lib/Status.tsx'
import { type AsyncState, useAsyncAction } from '@/lib/useAsyncAction.ts'
import { LabeledInput } from '@/lib/WriteForm.tsx'
import { useSetUp, useSetupOffer } from './setupApi.ts'

// What each place means for where the file applies.
const placeWords: Record<SetupPlaceName, string> = {
  repository: 'This repository: it applies in every directory in it.',
  home: 'Your home directory: it applies everywhere.',
}

// SetupValues are the setup form's answers.
type SetupValues = Omit<SetupRequest, 'keep_unchecked'>

// SetupArea is Settings where no configuration file applies: the questions
// `workflow config init` asks, once where the file may go is read.
export function SetupArea({ onWritten }: { onWritten: (result: SetupResult) => void }) {
  const query = useSetupOffer()

  if (query.isPending) {
    return <Reading>Reading where the file can go…</Reading>
  }

  if (query.isError) {
    return (
      <Unread
        reason={apiErrorMessage(query.error, 'Where the file can go could not be read.')}
        refusals={query.errorUpdateCount}
        retrying={query.isFetching}
        onRetry={() => {
          void query.refetch()
        }}
      />
    )
  }

  return <SetupForm offer={query.data} onWritten={onWritten} />
}

// SetupForm asks where the file goes, then Jira's address and token, checked
// with Jira before anything is written, the keychain where there is one, and
// a Slack webhook; then writes the file, which the server works with from
// then on.
function SetupForm({
  offer,
  onWritten,
}: {
  offer: SetupOffer
  onWritten: (result: SetupResult) => void
}) {
  const headingId = useId()
  const { register, handleSubmit, control, getValues } = useForm<SetupValues>({
    defaultValues: {
      place: offer.places[0]?.place ?? 'repository',
      jira_base_url: '',
      jira_token: '',
      webhook_url: '',
      keychain: offer.keychain,
    },
  })
  const setUp = useSetUp()
  const write = useAsyncAction(
    async (values: SetupValues, keepUnchecked: boolean) => {
      const keychain = offer.keychain && values.keychain
      onWritten(await setUp({ ...values, keychain, keep_unchecked: keepUnchecked }))
    },
    { fallback: 'The file was not written. Try again — your answers are still in the form.' },
  )

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-block">
      <div className="flex flex-col gap-item">
        <h2 id={headingId} className="text-base font-semibold">
          Set up workflow
        </h2>
        <p className="text-sm text-muted-foreground">
          No .workflow.json applies here yet. Answer what workflow config init asks, and workflow
          writes the file and works with it. Leave a question blank to skip it.
        </p>
      </div>
      <form
        aria-labelledby={headingId}
        onSubmit={(event) => {
          void handleSubmit((values) => write.run(values, false))(event)
        }}
        className="flex flex-col gap-section"
      >
        <PlaceChoice places={offer.places} register={register} />
        <JiraQuestions register={register} keychain={offer.keychain} />
        <SlackQuestion register={register} />
        <SetupWrite
          places={offer.places}
          control={control}
          state={write.state}
          error={write.error}
          code={write.code}
          onKeepAnyway={() => {
            void write.run(getValues(), true)
          }}
        />
      </form>
    </section>
  )
}

// Register is the setup form's own register, which each part is handed to
// put its fields in the form.
type Register = UseFormRegister<SetupValues>

// PlaceChoice asks where the file goes, saying where each place applies.
function PlaceChoice({ places, register }: { places: SetupPlace[]; register: Register }) {
  return (
    <fieldset className="flex flex-col gap-group">
      <legend className="mb-group text-base font-semibold">Where the file goes</legend>
      {places.map((place) => (
        <label key={place.place} className="flex items-start gap-item text-sm">
          <input type="radio" value={place.place} className="mt-1" {...register('place')} />
          <span className="flex min-w-0 flex-col">
            <span className="break-all">{place.shown}</span>
            <span className="text-xs text-muted-foreground">{placeWords[place.place]}</span>
          </span>
        </label>
      ))}
    </fieldset>
  )
}

// JiraQuestions ask for Jira's address and token, and offer the keychain
// where there is one.
function JiraQuestions({ register, keychain }: { register: Register; keychain: boolean }) {
  return (
    <fieldset className="flex flex-col gap-group">
      <legend className="mb-group text-base font-semibold">Jira</legend>
      <LabeledInput
        label="Address"
        hint="Such as https://jira.example.com. Blank leaves Jira out; the forge's issues are the tracker then."
      >
        {(id, hintId) => (
          <Input id={id} type="url" aria-describedby={hintId} {...register('jira_base_url')} />
        )}
      </LabeledInput>
      <LabeledInput
        label="Personal access token"
        hint="Checked with Jira before anything is written."
      >
        {(id, hintId) => (
          <Input
            id={id}
            type="password"
            autoComplete="off"
            aria-describedby={hintId}
            {...register('jira_token')}
          />
        )}
      </LabeledInput>
      {keychain ? <KeychainChoice register={register} /> : null}
    </fieldset>
  )
}

// SlackQuestion asks for a Slack incoming webhook, saved unchecked.
function SlackQuestion({ register }: { register: Register }) {
  return (
    <fieldset className="flex flex-col gap-group">
      <legend className="mb-group text-base font-semibold">Slack</legend>
      <LabeledInput
        label="Incoming webhook URL"
        hint="Saved unchecked: a webhook cannot be checked without posting. Blank posts with your Slack user token; run workflow slack login for it."
      >
        {(id, hintId) => (
          <Input
            id={id}
            type="password"
            autoComplete="off"
            aria-describedby={hintId}
            {...register('webhook_url')}
          />
        )}
      </LabeledInput>
    </fieldset>
  )
}

interface SetupWriteProps {
  places: SetupPlace[]
  control: Control<SetupValues>
  state: AsyncState
  error: string
  code: string
  onKeepAnyway: () => void
}

// SetupWrite is the write, named for the file it writes — or, under
// --dry-run, that it is held back.
function SetupWrite({ places, control, state, error, code, onKeepAnyway }: SetupWriteProps) {
  const dryRun = useHealthStore((health) => health.health?.dry_run === true)
  const [place, address] = useWatch({ control, name: ['place', 'jira_base_url'] })
  const chosen = places.find((each) => each.place === place)

  if (dryRun) {
    return (
      <p className="text-sm text-muted-foreground">
        Writing the file is held back while workflow runs with --dry-run.
      </p>
    )
  }

  return (
    <WriteControls
      act={`Write ${chosen?.shown ?? '.workflow.json'}`}
      busy={busyWords(state, address.trim() !== '')}
      error={state === 'error' ? error : ''}
      checkFailed={state === 'error' && code === 'check_failed'}
      onKeepAnyway={onKeepAnyway}
    />
  )
}

// busyWords is what the write says while it runs: that Jira is being asked,
// when it is, or that the file is being written; null while it is not running.
function busyWords(state: AsyncState, checking: boolean): string | null {
  if (state !== 'running') {
    return null
  }

  return checking ? 'Checking with Jira…' : 'Writing…'
}

// KeychainChoice offers to keep the token in the OS keychain, so the file
// holds only the command that reads it back.
function KeychainChoice({ register }: { register: Register }) {
  const hintId = useId()

  return (
    <div className="flex flex-col gap-tight">
      <label className="flex items-center gap-item text-sm">
        <input type="checkbox" aria-describedby={hintId} {...register('keychain')} />
        Keep the token in your keychain, out of the file
      </label>
      <p id={hintId} className="text-xs text-muted-foreground">
        Unchecked, the file keeps it, readable only by you.
      </p>
    </div>
  )
}

interface WriteControlsProps {
  act: string
  busy: string | null
  error: string
  checkFailed: boolean
  onKeepAnyway: () => void
}

// WriteControls are the write, what stopped the last one, and, when Jira's
// check is what stopped it, writing it anyway.
function WriteControls({ act, busy, error, checkFailed, onKeepAnyway }: WriteControlsProps) {
  return (
    <div className="flex flex-col items-start gap-item">
      <Button variant="primary" type="submit" disabled={busy !== null}>
        {busy ?? act}
      </Button>
      {error === '' ? null : <Failure>{error}</Failure>}
      {checkFailed ? (
        <Button variant="secondary" disabled={busy !== null} onClick={onKeepAnyway}>
          Write it anyway
        </Button>
      ) : null}
    </div>
  )
}
