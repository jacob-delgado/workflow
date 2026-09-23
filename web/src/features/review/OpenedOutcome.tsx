import type { FollowUp, OpenedPullRequest } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { linkOnIssue, moveToReview } from './followUpApi.ts'

// OpenedOutcome follows the line saying the pull request opened: the warning
// when its reviewers, assignees or labels could not all be added, and what the
// terminal and `workflow pr` offer next — to link it on the branch's issue,
// then to move that issue to the review status. The panel keeps it mounted
// above the pull request while its branch stays checked out, so the snapshot
// that brings the new pull request back does not take the offers, or what they
// said, away.
export function OpenedOutcome({ opened }: { opened: OpenedPullRequest }) {
  const { noun, sigil } = useForgeWords()
  const pull = `${sigil}${String(opened.pull.number)}`

  return (
    <div className="flex flex-col gap-2">
      {opened.warning === undefined || opened.warning === '' ? null : (
        <p role="status" className="text-sm text-warning">
          {opened.warning}
        </p>
      )}
      {opened.follow_ups.map((offer) => (
        <FollowUpOffer key={offer.action} offer={offer} pull={pull} noun={noun} />
      ))}
    </div>
  )
}

interface OfferWords {
  label: string
  busy: string
  done: string
  fallback: string
  act: () => Promise<void>
}

// offerWords is what an offer's button says, while it runs and once it is done,
// the fallback when a refusal gives no reason, and the write it makes.
function offerWords(offer: FollowUp, pull: string, noun: string): OfferWords {
  const key = offer.issue_key
  if (offer.action === 'link') {
    return {
      label: `Link it on ${key}`,
      busy: 'Linking…',
      done: `Linked ${pull} on ${key}.`,
      fallback: `The ${noun} was not linked on ${key}. Try again, or link it in Jira.`,
      act: () => linkOnIssue(key),
    }
  }

  const status = offer.status ?? ''

  return {
    label: `Move ${key} to ${status}`,
    busy: 'Moving…',
    done: `Moved ${key} to ${status}.`,
    fallback: `${key} was not moved to ${status}. Try again, or move it in Jira.`,
    act: () => moveToReview(key),
  }
}

// FollowUpOffer is one offer after opening: a button that makes the write, a
// live line that says it was made, and why when it was refused. Once the write
// is made the button goes — it cannot be made twice — and focus follows to what
// it said, unless the user has moved on.
function FollowUpOffer({ offer, pull, noun }: { offer: FollowUp; pull: string; noun: string }) {
  const words = offerWords(offer, pull, noun)
  const outcome = useOutcome()
  const { state, error, run } = useAsyncAction(words.act, {
    fallback: words.fallback,
    done: () => words.done,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col gap-1">
      {state === 'done' ? null : (
        <button
          type="button"
          disabled={state === 'running'}
          onClick={() => {
            void run()
          }}
          className="self-start rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
        >
          {state === 'running' ? words.busy : words.label}
        </button>
      )}
      <OutcomeLine said={outcome.said} />
      {state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}
