import { useRef, useState, type RefObject } from 'react'
import type { Branch, BranchIssuePreview } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { shownLinkKey } from '@/features/issues/issuePlaces.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { Input } from '@/lib/Field.tsx'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { linkIssue, previewLink, unlinkIssue } from './branchIssueApi.ts'

// IssueLink ties the branch to an issue by hand, for work begun outside
// workflow on a branch whose name names none: it names the issue a linked
// branch is for, with Unlink, and otherwise offers Link an issue. Focus
// follows each step to what replaces the control that took it.
export function IssueLink({ branch, outcome }: { branch: Branch; outcome: Teller }) {
  const [open, setOpen] = useState(false)
  const [held, setHeld] = useState<HeldLink>()
  const [offer, focusOffer] = useFocusHandback<HTMLButtonElement>()
  const linkKeys = useShortcut('link-issue', offer)
  const [unlinker, focusUnlink] = useFocusHandback<HTMLButtonElement>()
  const settle = (answered: Branch, focusNext: () => void) => {
    setHeld({ link: linkOf(answered), over: branch.issue_link, branch: branch.name })
    focusNext()
  }
  const link = linkShown(held, branch)

  if (link.issue_link !== '') {
    return (
      <Linked
        link={link}
        unlinker={unlinker}
        outcome={outcome}
        onUnlinked={(answered) => {
          settle(answered, focusOffer)
        }}
      />
    )
  }

  return open ? (
    <LinkForm
      outcome={outcome}
      onLinked={(answered) => {
        settle(answered, focusUnlink)
        setOpen(false)
      }}
      onClose={() => {
        focusOffer()
        setOpen(false)
      }}
    />
  ) : (
    <Button
      variant="secondary"
      ref={offer}
      aria-keyshortcuts={linkKeys}
      onClick={() => {
        setOpen(true)
      }}
    >
      Link an issue
    </Button>
  )
}

// LinkedIssue is the issue a branch is linked to, with the tracker the
// server names for it.
type LinkedIssue = Pick<Branch, 'issue_link' | 'issue_link_tracker'>

// linkOf is the branch's link, without the rest of the branch.
function linkOf(branch: Branch): LinkedIssue {
  return { issue_link: branch.issue_link, issue_link_tracker: branch.issue_link_tracker }
}

// HeldLink is the link a write answered with, over the link the stream
// showed when it answered, on the branch it answered for.
interface HeldLink {
  link: LinkedIssue
  over: string
  branch: string
}

// linkShown is the branch's link as the last write left it until the stream
// moves on from what it showed then: the stream can take seconds to report a
// write, and meanwhile the page would offer to make it again.
function linkShown(held: HeldLink | undefined, branch: Branch): LinkedIssue {
  return held?.branch === branch.name && held.over === branch.issue_link
    ? held.link
    : linkOf(branch)
}

interface LinkedProps {
  link: LinkedIssue
  unlinker: RefObject<HTMLButtonElement | null>
  outcome: Teller
  onUnlinked: (answered: Branch) => void
}

// Linked names the issue the branch is linked to, with Unlink, which stays
// focusable while it runs so a refusal finds focus where it was.
function Linked({ link, unlinker, outcome, onUnlinked }: LinkedProps) {
  const shown = shownLinkKey(link)
  const unlink = useAsyncAction(unlinkIssue, {
    fallback: 'The link was not forgotten. Try again.',
    done: (answered) => `Unlinked ${answered.name}.`,
    onStart: outcome.clear,
    onDone: (said, answered) => {
      outcome.say(said)
      onUnlinked(answered)
    },
  })

  return (
    <div className="flex flex-col gap-item">
      <div className="flex items-center gap-item text-sm">
        <span>
          Linked to <span className="font-mono">{shown}</span>
        </span>
        <Button
          variant="secondary"
          ref={unlinker}
          aria-label={`${unlink.state === 'running' ? 'Unlinking…' : 'Unlink'} ${shown}`}
          aria-disabled={unlink.state === 'running'}
          onClick={() => {
            if (unlink.state !== 'running') {
              void unlink.run()
            }
          }}
        >
          {unlink.state === 'running' ? 'Unlinking…' : 'Unlink'}
        </Button>
      </div>
      <Refusal message={refusalOf(unlink)} />
    </div>
  )
}

interface LinkFormProps {
  outcome: Teller
  onLinked: (answered: Branch) => void
  onClose: () => void
}

// LinkForm asks which issue, then — when the branch's pull request would
// change — shows its description with the issue's line before linking.
function LinkForm({ outcome, onLinked, onClose }: LinkFormProps) {
  const [key, setKey] = useState('')
  const [preview, setPreview] = useState<BranchIssuePreview>()
  // The key as it was asked about: the preview's own key is normalized, and
  // the link must name the issue whose description was shown.
  const asked = useRef('')
  const field = useFocusOnMount<HTMLInputElement>()
  const link = useAsyncAction(linkIssue, {
    fallback: 'The branch was not linked. Try again.',
    done: (answered) => `Linked ${answered.name} to ${shownLinkKey(answered)}.`,
    onStart: outcome.clear,
    onDone: (said, answered) => {
      outcome.say(said)
      onLinked(answered)
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

  // Linking asks for the preview, then links when no description changes.
  const linking = ask.state === 'running' || link.state === 'running'

  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault()
    asked.current = key.trim()
    void ask.run(asked.current)
  }

  return (
    <form onSubmit={submit} className="flex basis-full flex-col gap-item">
      <label className="flex flex-col gap-1 text-sm">
        Issue
        <Input
          ref={field}
          value={key}
          placeholder="PROJ-7 or #42"
          readOnly={ask.state === 'running'}
          onChange={(event) => {
            setKey(event.target.value)
            setPreview(undefined)
          }}
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
          <Button variant="primary" type="submit" held={linking}>
            {linking ? 'Linking…' : 'Link'}
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
