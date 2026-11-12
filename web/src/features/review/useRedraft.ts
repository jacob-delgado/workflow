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
// field is left naming another branch, filling only the fields still holding
// what was proposed before; a template chosen fills the description the same
// way. It answers what was last proposed, the base it was read past, and why
// a base could not be read.
export function useRedraft(
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
    // Only a read for the base the field still names is heard: one for a
    // base since left behind says nothing, answered or refused.
    const named = () => getValues('base').trim() === base
    previewPullRequest(template === '' ? undefined : template, base).then(
      (composed) => {
        if (named()) {
          fill(proposedBy(composed))
          setOnto(base)
        }
      },
      (caught: unknown) => {
        if (named()) {
          setError(
            apiErrorMessage(
              caught,
              `The commits past ${base} could not be read; the draft is kept.`,
            ),
          )
        }
      },
    )
  }

  const templated = (body: string) => {
    fill({ ...latest.current, body })
  }

  return { proposed, onto, error, ontoBase, templated }
}
