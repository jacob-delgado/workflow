import type { FollowUp, OpenedPullRequest } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { linkOnIssue, moveToReview } from './followUpApi.ts'
import { Failure } from '@/lib/Status.tsx'

// OpenedOutcome follows the line saying the pull request opened: the warning
// when its reviewers, assignees or labels could not all be added, and what the
// terminal and `workflow pr` offer next — to link it on the branch's issue,
// then to move that issue to the review status. The panel keeps it mounted
// above the pull request while its branch stays checked out, so the snapshot
// that brings the new pull request back does not take the offers, or what they
// said, away. It says, too, that the announcement was armed to post once CI
// passes, to the channel or the service named, or why a draft was not.
export function OpenedOutcome({ opened, service }: { opened: OpenedPullRequest; service: string }) {
  const { noun, sigil } = useForgeWords()
  const pull = `${sigil}${String(opened.pull.number)}`

  return (
    <div className="flex flex-col gap-item">
      {opened.warning === undefined || opened.warning === '' ? null : (
        <p role="status" className="text-sm text-warning">
          {opened.warning}
        </p>
      )}
      <ArmedLine opened={opened} service={service} />
      {opened.follow_ups.map((offer) => (
        <FollowUpOffer key={offer.action} offer={offer} pull={pull} noun={noun} />
      ))}
    </div>
  )
}

// ArmedLine says the opened pull request's announcement was armed, where it
// goes and how to drop it, or why none was: a draft is not announced.
function ArmedLine({ opened, service }: { opened: OpenedPullRequest; service: string }) {
  const armed = opened.announcement
  if (armed !== undefined) {
    const where = armed.channel === '' ? service : armed.channel

    return (
      <p role="status" className="text-sm">
        Announces to {where} once CI passes. To drop it, press Stop waiting in the {service}{' '}
        section.
      </p>
    )
  }

  if (opened.not_armed === undefined || opened.not_armed === '') {
    return null
  }

  return (
    <p role="status" className="text-sm text-muted-foreground">
      {sentence(opened.not_armed)}
    </p>
  )
}

// sentence is a reason the server words in lower case, as a sentence.
function sentence(reason: string): string {
  return reason.charAt(0).toUpperCase() + reason.slice(1) + '.'
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
    <div className="flex flex-col gap-tight">
      {state === 'done' ? null : (
        <Button
          variant="secondary"
          held={state === 'running'}
          onClick={() => {
            void run()
          }}
          className="self-start"
        >
          {state === 'running' ? words.busy : words.label}
        </Button>
      )}
      <OutcomeLine said={outcome.said} />
      {state === 'error' ? <Failure>{error}</Failure> : null}
    </div>
  )
}
