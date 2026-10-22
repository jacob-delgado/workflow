import { useState } from 'react'
import type { PullRequest, Snapshot } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { useLiveSnapshot } from '@/api/snapshot.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { cn, contentMeasure, definitionList } from '@/lib/utils.ts'
import { EmptyState } from '@/lib/EmptyState.tsx'
import { AnnounceControls } from './AnnounceControls.tsx'
import { HeldAnnouncement } from './HeldAnnouncement.tsx'

export function MessagingPanel() {
  const snapshot = useLiveSnapshot()

  const { messaging, review } = snapshot

  // A webhook service (Teams, Discord, a plain webhook) has no channel, so
  // channel presence cannot tell a configured destination from an unset one —
  // the snapshot carries `configured` for exactly that.
  if (!messaging.configured) {
    return (
      <EmptyState>
        {messaging.service} is not configured. Add a token or webhook in Settings.
      </EmptyState>
    )
  }

  return (
    <div className={cn('flex flex-col gap-section', contentMeasure)}>
      <dl className={definitionList}>
        <dt className="text-muted-foreground">Service</dt>
        <dd>{messaging.service}</dd>
        <dt className="text-muted-foreground">Channel</dt>
        <dd>{messaging.channel === '' ? '—' : messaging.channel}</dd>
        <dt className="text-muted-foreground">Announcing as</dt>
        <dd>{messaging.author === '' ? 'the webhook' : messaging.author}</dd>
      </dl>

      <AnnounceSection messaging={messaging} review={review} held={snapshot.queued_announcement} />

      {messaging.channels.length > 0 ? (
        <section aria-labelledby="channels-heading" className="flex flex-col gap-group">
          <h2 id="channels-heading" className="text-base font-semibold">
            Channels
          </h2>
          <ul className="flex flex-wrap gap-item">
            {messaging.channels.map((channel) => (
              <li key={channel} className="rounded-sm bg-muted px-2 py-1 text-xs">
                {channel}
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  )
}

// AnnounceSection offers to announce the branch's pull request, once there is
// one the loop follows and it was not announced at this moment already — from
// here, the terminal or workflow announce — and says where the announcement
// went, in a line that stays when the controls step aside. An announcement
// already made is said by the moment it was made at, once: not again where
// the section says it went already, in the held announcement's line or in its
// own outcome. One closed without merging is none the loop follows, as the
// server reads it: there is nothing to announce until one is open again.
// While a held announcement waits, its line leads and the controls follow it,
// as the other way to announce; the two are keyed, so the controls keep an
// open preview as the held one stops waiting and they change places.
function AnnounceSection({
  messaging,
  review,
  held,
}: {
  messaging: Snapshot['messaging']
  review: Snapshot['review']
  held: Snapshot['queued_announcement']
}) {
  const { noun, sigil } = useForgeWords()
  const outcome = useOutcome()
  const pull = pullFollowed(review)
  const announcing = useAnnouncedHere(outcome, pull === null ? '' : momentOf(pull))
  const waiting = held?.state === 'waiting'
  const controls =
    pull !== null && !review.announced ? (
      <AnnounceControls
        key="controls"
        pullName={`${sigil}${String(pull.number)}`}
        service={messaging.service}
        channels={messaging.channels}
        defaultChannel={messaging.channel}
        announcing={announcing}
        waiting={waiting}
      />
    ) : null
  const heldLine = (
    <HeldAnnouncement key="held" held={held} service={messaging.service} onSaid={outcome.say} />
  )

  return (
    <section aria-labelledby="announce-heading" className="flex flex-col gap-group">
      <h2 id="announce-heading" className="text-base font-semibold">
        Announce
      </h2>
      {pull !== null && review.announced ? (
        <AnnouncedAlready pull={pull} held={held} saidHere={announcing.saidHere} />
      ) : null}
      {pull === null ? (
        <p className="text-sm text-muted-foreground">
          Open a {noun} first — there is nothing to announce yet.
        </p>
      ) : null}
      {waiting ? [heldLine, controls] : [controls, heldLine]}
      <OutcomeLine said={outcome.said} />
    </section>
  )
}

// pullFollowed is the branch's pull request the loop follows: one found and
// not closed without merging, or null when there is none.
function pullFollowed(review: Snapshot['review']): PullRequest | null {
  return review.found && review.pull && review.pull.state !== 'closed' ? review.pull : null
}

// momentOf names the moment pull stands at, which an announcement is made at:
// its number and its state.
function momentOf(pull: PullRequest): string {
  return `${String(pull.number)} ${pull.state}`
}

// useAnnouncedHere is how the section says where an announcement made here
// went: start marks one going out at the moment the pull request stands at,
// say puts what became of it in the section's outcome, and refuse lets go of
// one refused. saidHere is whether the section says it already, for the
// moment the pull request stands at now: it is still going out — its answer
// handed on after the frame that agrees with it — or the outcome says it
// still, which no later write has cleared.
function useAnnouncedHere(outcome: ReturnType<typeof useOutcome>, moment: string) {
  const [announced, setAnnounced] = useState<{ text: string | null; moment: string }>({
    text: '',
    moment: '',
  })
  const said = announced.text === null || outcome.said?.text === announced.text

  return {
    start: () => {
      setAnnounced({ text: null, moment })
    },
    refuse: () => {
      setAnnounced({ text: '', moment: '' })
    },
    say: (text: string) => {
      setAnnounced({ text, moment })
      outcome.say(text)
    },
    saidHere: announced.moment === moment && said,
  }
}

// AnnouncedAlready says pull was announced at its moment already, unless the
// section says so already: in its own outcome, for one announced here at this
// moment, or in the line of the announcement held for pull's CI, which went
// once it passed — a hold is only ever for the moment it is ready for review.
function AnnouncedAlready({
  pull,
  held,
  saidHere,
}: {
  pull: PullRequest
  held: Snapshot['queued_announcement']
  saidHere: boolean
}) {
  const { sigil } = useForgeWords()
  const heldWent = held?.state === 'announced' && held.pull === pull.number
  if (saidHere || (pull.state === 'open' && heldWent)) {
    return null
  }

  return <p className="text-sm text-muted-foreground">{announcedAt(pull, sigil)}</p>
}

// announcedAt says pull was announced at the moment it stands at, as the
// sigil names it: ready for review while it is open, or merged. A pull request
// closed without merging has no moment to announce, and is never shown here
// (AnnounceSection), so it reads as one ready for review.
function announcedAt(pull: PullRequest, sigil: string): string {
  const name = `${sigil}${String(pull.number)}`

  switch (pull.state) {
    case 'merged':
      return `Announced the merge of ${name}.`
    case 'open':
    case 'closed':
      return `Announced ${name} as ready for review.`
  }
}
