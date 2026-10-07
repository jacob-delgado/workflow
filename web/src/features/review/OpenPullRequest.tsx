import { useEffect, useState } from 'react'
import { useForm, useWatch, type UseFormRegister } from 'react-hook-form'
import { apiErrorMessage } from '@/api/apiError.ts'
import { useForgeWords } from '@/api/health.ts'
import type {
  OpenedPullRequest,
  OpenPullRequestRequest,
  PullRequestDraft,
} from '@/api/generated/types.gen.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { Input, Select, TextArea } from '@/lib/Field.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import { Failure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { splitList } from '@/lib/utils.ts'
import { openPr, previewPullRequest } from './openPrApi.ts'

// OpenPullRequest opens a pull request for a branch that has none yet, behind a
// preview: it composes the proposal, shows it as an editable form, and opens on
// confirm — pushing the branch first when it is not yet published. Composing and
// opening are two steps, each its own action. On success it hands what the open
// answered, and what to say of it, to the panel, which says so above it, and
// steps aside until the event stream brings back the new pull request.
export function OpenPullRequest({
  onOpened,
}: {
  onOpened: (opened: OpenedPullRequest, said: string) => void
}) {
  const { noun, sigil } = useForgeWords()
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const openKeys = useShortcut('open-pull-request', opener)
  const compose = useAsyncAction(() => previewPullRequest(), {
    fallback: `The ${noun} could not be composed. Try again, or run workflow pr from a terminal.`,
  })
  const open = useAsyncAction(openPr, {
    fallback: `The ${noun} was not opened. Try again — your edits are still in the form.`,
    done: (opened) => `Opened ${noun} ${sigil}${String(opened.pull.number)}.`,
    onDone: (said, opened) => {
      onOpened(opened, said)
    },
  })

  if (open.state === 'done') {
    return null
  }

  // A failed open keeps the form up with its reason, so the edits are not lost;
  // a failed compose has no draft to edit, so it falls through to the retry
  // button below.
  if (compose.state === 'done' && compose.result !== undefined) {
    return (
      <PullRequestForm
        draft={compose.result}
        opening={open.state === 'running'}
        error={open.state === 'error' ? open.error : ''}
        onCancel={() => {
          handBack()
          open.reset()
          compose.reset()
        }}
        onSubmit={(request) => {
          void open.run(request)
        }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-item">
      <p className="text-sm text-muted-foreground">No open {noun} for this branch yet.</p>
      <Button
        variant="primary"
        ref={opener}
        aria-keyshortcuts={openKeys}
        held={compose.state === 'running'}
        onClick={() => {
          void compose.run()
        }}
        className="self-start"
      >
        {compose.state === 'running' ? 'Preparing…' : `Open a ${noun}`}
      </Button>
      {compose.state === 'error' ? (
        <p role="alert" className="text-sm whitespace-pre-line text-destructive">
          {compose.error}
        </p>
      ) : null}
    </div>
  )
}

interface PullRequestFields {
  title: string
  base: string
  body: string
  draft: boolean
  reviewers: string
  assignees: string
  labels: string
}

// PullRequestForm is the composed pull request, editable, opening on its title,
// its reviewers pre-filled with the code owners the draft proposes, with a
// confirm that opens it and a cancel. Its buttons are held while the open is in
// flight, so a second click cannot open a second pull request.
function PullRequestForm({
  draft,
  opening,
  error,
  onCancel,
  onSubmit,
}: {
  draft: PullRequestDraft
  opening: boolean
  error: string
  onCancel: () => void
  onSubmit: (request: OpenPullRequestRequest) => void
}) {
  const { noun } = useForgeWords()
  const { register, handleSubmit, setFocus, setValue, control } = useForm<PullRequestFields>({
    defaultValues: {
      title: draft.title,
      base: draft.base,
      body: draft.body,
      draft: draft.draft,
      reviewers: draft.reviewers.join(', '),
      assignees: '',
      labels: '',
    },
  })

  // The form appears only because it was asked for, so focus goes with the
  // person asking to its first field.
  useEffect(() => {
    setFocus('title')
  }, [setFocus])

  const body = useWatch({ control, name: 'body' })
  const submit = handleSubmit((fields) => {
    onSubmit(proposal(fields))
  })

  return (
    <form
      aria-label={`Open a ${noun}`}
      onSubmit={(event) => {
        void submit(event)
      }}
      className="flex flex-col gap-group rounded-lg border border-border p-4"
    >
      <ProposalFields register={register} />

      <TemplateChoice
        draft={draft}
        body={body}
        onBody={(body) => {
          setValue('body', body)
        }}
      />

      {draft.needs_push ? (
        <p className="text-xs text-muted-foreground">
          The branch is not pushed yet; opening will push it first.
        </p>
      ) : null}

      <div className="flex items-center gap-item">
        <Button variant="secondary" held={opening} onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" held={opening}>
          {opening ? 'Opening…' : `Open ${noun}`}
        </Button>
      </div>

      {error === '' ? null : (
        <p role="alert" className="text-sm whitespace-pre-line text-destructive">
          {error}
        </p>
      )}
    </form>
  )
}

// proposal is the request the form's fields make, each comma-separated list
// read into its trimmed entries.
function proposal(fields: PullRequestFields): OpenPullRequestRequest {
  return {
    title: fields.title,
    base: fields.base,
    body: fields.body,
    draft: fields.draft,
    reviewers: splitList(fields.reviewers),
    assignees: splitList(fields.assignees),
    labels: splitList(fields.labels),
  }
}

// ProposalFields are what a pull request opens with, each editable: its title
// and base, the people and labels it asks for, its description, and whether
// it opens as a draft.
function ProposalFields({ register }: { register: UseFormRegister<PullRequestFields> }) {
  return (
    <>
      <label className={labelClass}>
        Title
        <Input {...register('title')} required />
      </label>

      <label className={labelClass}>
        Base branch
        <Input {...register('base')} required />
      </label>

      <label className={labelClass}>
        Reviewers
        <Input {...register('reviewers')} placeholder="Comma-separated usernames or org/team" />
      </label>

      <label className={labelClass}>
        Assignees
        <Input {...register('assignees')} placeholder="Comma-separated usernames" />
      </label>

      <label className={labelClass}>
        Labels
        <Input {...register('labels')} placeholder="Comma-separated labels" />
      </label>

      <label className={labelClass}>
        Description
        <TextArea {...register('body')} rows={6} />
      </label>

      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" {...register('draft')} className="size-4" />
        Open as a draft
      </label>
    </>
  )
}

const labelClass = 'flex flex-col gap-tight text-sm text-muted-foreground'

// TemplateChoice starts the description from another of the repository's
// templates, as the terminal's ctrl+t does, while there are several and the
// description is not yet edited — a template never overwrites an edit.
function TemplateChoice({
  draft,
  body,
  onBody,
}: {
  draft: PullRequestDraft
  body: string
  onBody: (body: string) => void
}) {
  const [chosen, setChosen] = useState(draft.template)
  const [applied, setApplied] = useState(draft.body)
  const [error, setError] = useState('')
  const edited = body !== applied

  if (draft.templates.length < 2) {
    return null
  }

  return (
    <div className="flex flex-col gap-tight">
      <label className={labelClass}>
        Template
        <Select
          value={chosen}
          disabled={edited}
          aria-describedby={edited ? 'template-kept' : undefined}
          onChange={(event) => {
            const name = event.target.value
            setChosen(name)
            setError('')
            previewPullRequest(name).then(
              (composed) => {
                setApplied(composed.body)
                onBody(composed.body)
              },
              (caught: unknown) => {
                setError(
                  apiErrorMessage(caught, `The ${name} template could not be read. Try again.`),
                )
              },
            )
          }}
        >
          {draft.templates.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </Select>
      </label>
      {edited ? (
        <span id="template-kept" className="text-xs text-muted-foreground">
          The description is edited; a template no longer replaces it.
        </span>
      ) : null}
      {error === '' ? null : <Failure>{error}</Failure>}
    </div>
  )
}
