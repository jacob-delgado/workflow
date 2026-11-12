import { useRef, useState } from 'react'
import type { UseFormGetValues, UseFormSetValue } from 'react-hook-form'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { PullRequestDraft } from '@/api/generated/types.gen.ts'
import { previewPullRequest } from './openPrApi.ts'

// PullRequestFields are what the form to open a pull request holds, each
// field as typed.
export interface PullRequestFields {
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
export function proposedBy(draft: PullRequestDraft): Proposed {
  return { title: draft.title, body: draft.body, reviewers: draft.reviewers.join(', ') }
}

// useRedraft drafts the form again past the base its field names once that
// field is left naming another branch, and with the template chosen once one
// is, filling only the fields still holding what was proposed before. Only
// the read begun last is heard, base or template alike, and a template is
// read past the base the last base read went to, so no read fills the form
// from a choice since left behind. It answers what was last proposed, the
// base it was read past, the template chosen, and why a base or a template
// could not be read.
export function useRedraft(
  draft: PullRequestDraft,
  getValues: UseFormGetValues<PullRequestFields>,
  setValue: UseFormSetValue<PullRequestFields>,
) {
  const [proposed, setProposed] = useState(() => proposedBy(draft))
  // latest is what was last proposed as an answer lands, which a read begun
  // before another landed must compare against, not what it saw.
  const latest = useRef(proposed)
  const [onto, setOnto] = useState(draft.base)
  const [template, setTemplate] = useState(draft.template)
  const [error, setError] = useState('')
  const [templateError, setTemplateError] = useState('')
  // reads numbers every read begun, so only the last one's answer is heard;
  // aim is the base that read goes past.
  const reads = useRef(0)
  const aim = useRef(draft.base)

  const fill = (next: Proposed) => {
    const now = getValues()
    for (const field of ['title', 'body', 'reviewers'] as const) {
      if (now[field] === latest.current[field]) {
        setValue(field, next[field])
      }
    }
    latest.current = next
    setProposed(next)
  }

  const read = (chosen: string, base: string, refused: (caught: unknown) => void) => {
    reads.current += 1
    const turn = reads.current
    aim.current = base
    previewPullRequest(chosen === '' ? undefined : chosen, base).then(
      (composed) => {
        if (turn === reads.current) {
          fill(proposedBy(composed))
          setOnto(base)
        }
      },
      (caught: unknown) => {
        if (turn === reads.current) {
          aim.current = onto
          refused(caught)
        }
      },
    )
  }

  // ontoBase reads again past the base the field names, or past the base
  // drafted onto when it is left blank, unless the last read goes there.
  const ontoBase = () => {
    const base = getValues('base').trim() || onto
    setError('')
    if (base === aim.current) {
      return
    }
    read(template, base, (caught) => {
      setError(
        apiErrorMessage(caught, `The commits past ${base} could not be read; the draft is kept.`),
      )
    })
  }

  const choose = (name: string) => {
    setTemplate(name)
    setTemplateError('')
    read(name, aim.current, (caught) => {
      setTemplateError(
        apiErrorMessage(caught, `The ${name} template could not be read. Try again.`),
      )
    })
  }

  return { proposed, onto, template, error, templateError, ontoBase, choose }
}
