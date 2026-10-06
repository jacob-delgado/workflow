import type { QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { Button } from '@/lib/Button.tsx'
import { Failure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { stopWaiting } from './announceApi.ts'

interface HeldAnnouncementProps {
  held: QueuedAnnouncement | undefined
  service: string
  onSaid: (said: string) => void
}

// HeldAnnouncement says how the announcement held until CI passes stands, as
// the server's stream tells it: waiting, with Stop waiting; going; gone; or
// dropped, with why, as a failure. The server holds it while it runs, so it
// stands whether or not this page was open when it was held.
export function HeldAnnouncement({ held, service, onSaid }: HeldAnnouncementProps) {
  const { sigil } = useForgeWords()
  if (held === undefined) {
    return null
  }

  const where = held.channel === '' ? service : held.channel

  switch (held.state) {
    case 'waiting':
      return (
        <Waiting
          said={`Waiting for CI on ${sigil}${String(held.pull)} to pass, then announcing to ${where}.`}
          onSaid={onSaid}
        />
      )
    case 'announcing':
      return <p className="text-sm text-muted-foreground">Announcing to {where}…</p>
    case 'announced':
      return <p className="text-sm">Announced to {where} once CI passed.</p>
    case 'dropped':
      return <Failure>Not announced: {held.reason ?? 'it was dropped'}.</Failure>
  }
}

// Waiting is a held announcement still waiting for CI, with Stop waiting,
// which drops it unposted. Stop waiting stays off once pressed, until the
// stream no longer shows it waiting.
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
