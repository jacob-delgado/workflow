import { useSnapshotStore } from '@/api/snapshot.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'

export function SlackPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  if (!snapshot) {
    return <EmptyState>Connecting…</EmptyState>
  }

  const { slack } = snapshot

  if (slack.channel === '' && slack.channels.length === 0) {
    return <EmptyState>Slack is not configured. Add a token or webhook in Settings.</EmptyState>
  }

  return (
    <div className="mt-4 flex max-w-2xl flex-col gap-6">
      <dl className="grid grid-cols-[8rem_1fr] gap-x-4 gap-y-1.5 text-sm">
        <dt className="text-muted-foreground">Channel</dt>
        <dd className="font-mono">{slack.channel === '' ? '—' : slack.channel}</dd>
        <dt className="text-muted-foreground">Posting as</dt>
        <dd>{slack.author === '' ? 'the webhook' : slack.author}</dd>
      </dl>

      {slack.channels.length > 0 ? (
        <section aria-labelledby="channels-heading" className="flex flex-col gap-2">
          <h2
            id="channels-heading"
            className="text-sm font-semibold text-muted-foreground uppercase"
          >
            Channels
          </h2>
          <ul className="flex flex-wrap gap-2">
            {slack.channels.map((channel) => (
              <li key={channel} className="rounded-md bg-muted px-2 py-1 font-mono text-xs">
                {channel}
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  )
}
