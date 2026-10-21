import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import {
  useForm,
  useFormState,
  useWatch,
  type Control,
  type UseFormGetValues,
  type UseFormRegister,
  type UseFormRegisterReturn,
  type UseFormSetValue,
} from 'react-hook-form'
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
      {compose.state === 'error' ? <Failure>{compose.error}</Failure> : null}
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

// Proposed is what the form last proposed for the fields a draft fills, so a
// draft read again replaces only those still standing as proposed.
interface Proposed {
  title: string
  body: string
  reviewers: string
}

// proposedBy is what a draft proposes for the fields it fills.
function proposedBy(draft: PullRequestDraft): Proposed {
  return { title: draft.title, body: draft.body, reviewers: draft.reviewers.join(', ') }
}

// PullRequestForm is the composed pull request, editable, opening on its title,
// its reviewers pre-filled with the code owners the draft proposes, with a
// confirm that opens it and a cancel. Leaving the base field naming another
// branch drafts it again past that base, replacing only what is not yet
// edited. Its buttons are held while the open is in flight, so a second click
// cannot open a second pull request.
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
  const { register, handleSubmit, setFocus, setValue, getValues, control } =
    useForm<PullRequestFields>({
      defaultValues: {
        ...proposedBy(draft),
        base: draft.base,
        draft: draft.draft,
        assignees: '',
        labels: '',
      },
    })
  const [template, setTemplate] = useState(draft.template)
  const redraft = useRedraft(draft, template, getValues, setValue)

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
      noValidate
      onSubmit={(event) => {
        void submit(event)
      }}
      className="flex flex-col gap-group rounded-lg border border-border p-4"
    >
      <ProposalFields register={register} control={control} onBaseLeft={redraft.ontoBase} />

      {redraft.error === '' ? null : <Failure>{redraft.error}</Failure>}

      <TemplateChoice
        templates={draft.templates}
        chosen={template}
        base={redraft.onto}
        edited={body !== redraft.proposed.body}
        onChoose={setTemplate}
        onBody={redraft.templated}
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

      {error === '' ? null : <Failure>{error}</Failure>}
    </form>
  )
}

// useRedraft drafts the form again past the base its field names once that
// field is left naming another branch, filling only the fields still holding
// what was proposed before; a template chosen fills the description the same
// way. It answers what was last proposed, the base it was read past, and why
// a base could not be read.
function useRedraft(
  draft: PullRequestDraft,
  template: string,
  getValues: UseFormGetValues<PullRequestFields>,
  setValue: UseFormSetValue<PullRequestFields>,
) {
  const [proposed, setProposed] = useState(() => proposedBy(draft))
  // latest is what was last proposed as an answer lands, which a read begun
  // before a template was chosen must compare against, not what it saw.
  const latest = useRef(proposed)
  const [onto, setOnto] = useState(draft.base)
  const [error, setError] = useState('')

  const propose = (next: Proposed) => {
    latest.current = next
    setProposed(next)
  }

  const fill = (next: Proposed) => {
    const now = getValues()
    for (const field of ['title', 'body', 'reviewers'] as const) {
      if (now[field] === latest.current[field]) {
        setValue(field, next[field])
      }
    }
    propose(next)
  }

  const ontoBase = () => {
    const base = getValues('base').trim()
    setError('')
    if (base === '' || base === onto) {
      return
    }
    previewPullRequest(template === '' ? undefined : template, base).then(
      (composed) => {
        if (getValues('base').trim() === base) {
          fill(proposedBy(composed))
          setOnto(base)
        }
      },
      (caught: unknown) => {
        setError(
          apiErrorMessage(caught, `The commits past ${base} could not be read; the draft is kept.`),
        )
      },
    )
  }

  const templated = (body: string) => {
    setValue('body', body)
    propose({ ...latest.current, body })
  }

  return { proposed, onto, error, ontoBase, templated }
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
function ProposalFields({
  register,
  control,
  onBaseLeft,
}: {
  register: UseFormRegister<PullRequestFields>
  control: Control<PullRequestFields>
  onBaseLeft: () => void
}) {
  return (
    <>
      <NeededField
        label="Title"
        field={register('title', { validate: typed('Type a title first.') })}
        control={control}
      />

      <NeededField
        label="Base branch"
        field={register('base', {
          onBlur: onBaseLeft,
          validate: typed('Name the branch to merge into first.'),
        })}
        control={control}
      />

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

      <DescriptionField register={register} control={control} />

      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" {...register('draft')} className="size-4" />
        Open as a draft
      </label>
    </>
  )
}

// DescriptionField is the pull request's body, grown to the whole of its text
// up to a bound, so the template it starts from is never cut through a line.
function DescriptionField({
  register,
  control,
}: {
  register: UseFormRegister<PullRequestFields>
  control: Control<PullRequestFields>
}) {
  const field = register('body')
  const fitted = useFitsItsText(useWatch({ control, name: 'body' }), field.ref)

  return (
    <label className={labelClass}>
      Description
      <TextArea {...field} ref={fitted} rows={6} className="max-h-96" />
    </label>
  )
}

// useFitsItsText grows a text area to the text it holds, measured again each
// time the text changes or the window does, which can wrap it anew; its rows
// stay its least height, and its own max-height its most. It answers the ref
// to put on the text area, which hands the element on to the form's own.
function useFitsItsText(text: string, formRef: (area: HTMLTextAreaElement | null) => void) {
  const box = useRef<HTMLTextAreaElement | null>(null)

  useLayoutEffect(() => {
    const fit = () => {
      const area = box.current
      if (area === null) {
        return
      }

      area.style.height = 'auto'
      const borders = area.offsetHeight - area.clientHeight
      area.style.height = `${String(area.scrollHeight + borders)}px`
    }
    fit()
    window.addEventListener('resize', fit)

    return () => {
      window.removeEventListener('resize', fit)
    }
  }, [text])

  return (area: HTMLTextAreaElement | null) => {
    formRef(area)
    box.current = area
  }
}

const labelClass = 'flex flex-col gap-tight text-sm text-muted-foreground'

// typed refuses a field left empty, with the words given.
function typed(refusal: string) {
  return (value: string): true | string => (value.trim() === '' ? refusal : true)
}

interface NeededFieldProps {
  label: string
  field: UseFormRegisterReturn<'title' | 'base'>
  control: Control<PullRequestFields>
}

// NeededField is a field a pull request cannot open without, refused beside
// itself when left empty.
function NeededField({ label, field, control }: NeededFieldProps) {
  const refusalId = useId()
  const { errors } = useFormState({ control, name: field.name })
  const refused = errors[field.name]?.message

  return (
    <div className="flex flex-col gap-tight">
      <label className={labelClass}>
        {label}
        <Input {...field} required error={refused === undefined ? undefined : refusalId} />
      </label>
      {refused === undefined ? null : <Failure id={refusalId}>{refused}</Failure>}
    </div>
  )
}

// TemplateChoice starts the description from another of the repository's
// templates, as the terminal's ctrl+t does, while there are several and the
// description is not yet edited — a template never overwrites an edit. The
// template is read past the base the form drafts onto.
function TemplateChoice({
  templates,
  chosen,
  base,
  edited,
  onChoose,
  onBody,
}: {
  templates: string[]
  chosen: string
  base: string
  edited: boolean
  onChoose: (template: string) => void
  onBody: (body: string) => void
}) {
  const [error, setError] = useState('')

  if (templates.length < 2) {
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
            onChoose(name)
            setError('')
            previewPullRequest(name, base).then(
              (composed) => {
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
          {templates.map((name) => (
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
