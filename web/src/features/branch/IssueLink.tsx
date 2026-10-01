import { useRef, useState } from 'react'
import type { Branch, BranchIssuePreview } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { shownLinkKey } from '@/features/issues/issuePlaces.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { linkIssue, previewLink, unlinkIssue } from './branchIssueApi.ts'

const inputClass =
  'rounded-md border border-input bg-transparent px-3 py-2 text-sm text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'

// IssueLink ties the branch to an issue by hand, for work begun outside
// workflow on a branch whose name names none: it names the issue a linked
// branch is for, with Unlink, and otherwise offers Link an issue.
export function IssueLink({ branch, outcome }: { branch: Branch; outcome: Teller }) {
  const [open, setOpen] = useState(false)
  const unlink = useAsyncAction(unlinkIssue, {
    fallback: 'The link was not forgotten. Try again.',
    done: (answered) => `Unlinked ${answered.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  if (branch.issue_link !== '') {
    const shown = shownLinkKey(branch.issue_link)

    return (
      <div className="flex items-center gap-item text-sm">
        <span>
          Linked to <span className="font-mono">{shown}</span>
        </span>
        <Button
          variant="secondary"
          aria-label={`Unlink ${shown}`}
          disabled={unlink.state === 'running'}
          onClick={() => void unlink.run()}
        >
          Unlink
        </Button>
      </div>
    )
  }

  return open ? (
    <LinkForm
      outcome={outcome}
      onClose={() => {
        setOpen(false)
      }}
    />
  ) : (
    <Button
      variant="secondary"
      className="self-start"
      onClick={() => {
        setOpen(true)
      }}
    >
      Link an issue
    </Button>
  )
}

// LinkForm asks which issue, then — when the branch's pull request would
// change — shows its description with the issue's line before linking.
function LinkForm({ outcome, onClose }: { outcome: Teller; onClose: () => void }) {
  const [key, setKey] = useState('')
  const [preview, setPreview] = useState<BranchIssuePreview>()
  // The key as it was asked about: the preview's own key is normalized, and
  // the link must name the issue whose description was shown.
  const asked = useRef('')
  const field = useFocusOnMount<HTMLInputElement>()
  const link = useAsyncAction(linkIssue, {
    fallback: 'The branch was not linked. Try again.',
    done: (answered) => `Linked ${answered.name} to ${shownLinkKey(answered.issue_link)}.`,
    onStart: outcome.clear,
    onDone: (said) => {
      outcome.say(said)
      onClose()
    },
  })
  const ask = useAsyncAction(previewLink, {
    fallback: 'That names no issue: a Jira key like PROJ-7, or a forge number like #42.',
    onDone: (_, shown) => {
      if (shown.changes && shown.pull > 0) {
        setPreview(shown)
      } else {
        void link.run(asked.current, false)
      }
    },
  })

  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault()
    asked.current = key.trim()
    void ask.run(asked.current)
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-item">
      <label className="flex flex-col gap-1 text-sm">
        Issue
        <input
          ref={field}
          value={key}
          placeholder="PROJ-7 or #42"
          readOnly={ask.state === 'running'}
          onChange={(event) => {
            setKey(event.target.value)
            setPreview(undefined)
          }}
          className={inputClass}
        />
      </label>
      {preview ? (
        <DescriptionChange
          preview={preview}
          onLink={(update) => void link.run(asked.current, update)}
        />
      ) : null}
      <div className="flex gap-item">
        {preview ? null : (
          <Button variant="primary" type="submit" disabled={ask.state === 'running'}>
            Link
          </Button>
        )}
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
      </div>
      <Refusal message={refusalOf(ask, link)} />
    </form>
  )
}

// DescriptionChange is the pull request's description as linking would leave
// it, with the choice to update it or to link alone.
function DescriptionChange({
  preview,
  onLink,
}: {
  preview: BranchIssuePreview
  onLink: (update: boolean) => void
}) {
  const { sigil } = useForgeWords()
  const mark = `${sigil}${String(preview.pull)}`

  return (
    <div className="flex flex-col gap-item text-sm">
      <p>{mark}&apos;s description becomes:</p>
      <pre className="rounded-md border border-border p-3 whitespace-pre-wrap">{preview.body}</pre>
      <div className="flex gap-item">
        <Button
          variant="primary"
          onClick={() => {
            onLink(true)
          }}
        >
          Link and update {mark}
        </Button>
        <Button
          variant="secondary"
          onClick={() => {
            onLink(false)
          }}
        >
          Link only
        </Button>
      </div>
    </div>
  )
}

// Refusal says why a step was refused, when one was.
function Refusal({ message }: { message: string }) {
  if (message === '') {
    return null
  }

  return (
    <p role="alert" className="text-sm whitespace-pre-line text-destructive">
      {message}
    </p>
  )
}

// refusalOf is why the preview or the link was refused, when either was.
function refusalOf(...steps: { state: string; error: string }[]): string {
  return steps.find((step) => step.state === 'error')?.error ?? ''
}
