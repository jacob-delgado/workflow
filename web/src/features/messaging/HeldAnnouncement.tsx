import type { QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { Button } from '@/lib/Button.tsx'
import { Failure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { stopWaiting } from './announceApi.ts'
import { withWarning } from './withWarning.ts'

interface HeldAnnouncementProps {
  held: QueuedAnnouncement | undefined
  service: string
  onSaid: (said: string) => void
}

// HeldAnnouncement says how the announcement held until CI passes stands, as
// the server's stream tells it: waiting, with Stop waiting; going; gone, and
// that it is not offered again when the store could not confirm it; or
// dropped, with why, as a failure. The server holds it while it runs, so it
// stands whether or not this page was open when it was held.
export function HeldAnnouncement({ held, service, onSaid }: HeldAnnouncementProps) {
  const { sigil } = useForgeWords()
  if (held === undefined) {
    return null
  }

  const where = held.channel === '' ? service : held.channel

  if (held.state === 'waiting') {
    return (
      <Waiting
        key={held.id}
        said={armedSaying(held, `${sigil}${String(held.pull)}`, where)}
        onSaid={onSaid}
      />
    )
  }

  return <Settled held={held} where={where} />
}

// Settled says how a held announcement no longer waiting stands: going; gone,
// and that it is not offered again when the store could not confirm it; not
// posted here, another session having claimed it first; or dropped, with why,
// as a failure. One stopped says nothing: Stop waiting says so in the
// section's outcome where it was pressed.
function Settled({ held, where }: { held: QueuedAnnouncement; where: string }) {
  switch (held.state) {
    case 'announcing':
      return <p className="text-sm text-muted-foreground">Announcing to {where}…</p>
    case 'announced':
      return (
        <p className="text-sm">
          {withWarning(`Announced to ${where} once CI passed.`, held.warning)}
        </p>
      )
    case 'elsewhere':
      return (
        <p className="text-sm text-muted-foreground">
          Not posted here: {held.reason ?? 'it was announced from another session'}.
        </p>
      )
    case 'dropped':
      return <Failure>Not announced: {held.reason ?? 'it was dropped'}.</Failure>
    case 'waiting':
    case 'stopped':
      return null
  }
}

// armedSaying is what a held announcement waiting says: that it was armed as
// its pull request opened, when it was, then what it waits on.
function armedSaying(held: QueuedAnnouncement, pull: string, where: string): string {
  const waits = waitingOn(held, pull, where)

  return held.automatic === true ? `Armed when ${pull} opened. ${waits}` : waits
}

// waitingOn says what the held announcement of pull, to where, waits on, as
// the server says: the forge to list a pull request just opened; a fix for a
// check that failed; checks to be reported — a repository with no CI is
// announced by hand; a read or a post that failed, tried again, and why; or
// the CI to pass.
function waitingOn(held: QueuedAnnouncement, pull: string, where: string): string {
  switch (held.waiting_on) {
    case 'listing':
      return `Waiting for the forge to list ${pull}, then announcing to ${where} once CI passes.`
    case 'fix':
      return `A check failed on ${pull}; waiting on a fix, then announcing to ${where} once CI passes.`
    case 'checks':
      return (
        `Waiting for checks to be reported on ${pull}, then announcing to ${where} once CI passes. ` +
        'A repository with no CI is announced by hand.'
      )
    case 'unread':
      return (
        `Not announced yet: ${held.reason ?? 'a read failed'}. Trying again, then announcing to ` +
        `${where} once CI passes.`
      )
    case 'ci':
    case undefined:
      return `Waiting for CI on ${pull} to pass, then announcing to ${where}.`
  }
}

// Waiting is a held announcement still waiting for CI, with Stop waiting,
// which stops it unposted. Stop waiting stays off once pressed, until the
// stream no longer shows it waiting; keyed by the hold's id, one held afresh
// meanwhile offers it again.
function Waiting({ said, onSaid }: { said: string; onSaid: (said: string) => void }) {
  const stop = useAsyncAction(stopWaiting, {
    fallback: 'The announcement is still waiting. Try again.',
    done: () => 'Stopped waiting; nothing was announced.',
    onDone: onSaid,
  })

  return (
    <div className="flex flex-col items-start gap-item">
      <p className="text-sm">{said}</p>
      <Button
        variant="secondary"
        held={stop.state === 'running' || stop.state === 'done'}
        onClick={() => {
          void stop.run()
        }}
      >
        {stop.state === 'running' ? 'Stopping…' : 'Stop waiting'}
      </Button>
      {stop.state === 'error' ? <Failure>{stop.error}</Failure> : null}
    </div>
  )
}
