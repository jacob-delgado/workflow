import { useState } from 'react'
import type { Snapshot } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { announce, previewAnnouncement } from './announceApi.ts'

export function MessagingPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  // The shell says it is connecting until the first snapshot lands.
  if (!snapshot) {
    return null
  }

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
    <div className="flex max-w-2xl flex-col gap-section">
      <dl className="grid grid-cols-[8rem_1fr] gap-x-group gap-y-tight text-sm">
        <dt className="text-muted-foreground">Service</dt>
        <dd>{messaging.service}</dd>
        <dt className="text-muted-foreground">Channel</dt>
        <dd>{messaging.channel === '' ? '—' : messaging.channel}</dd>
        <dt className="text-muted-foreground">Announcing as</dt>
        <dd>{messaging.author === '' ? 'the webhook' : messaging.author}</dd>
      </dl>

      <AnnounceSection messaging={messaging} found={review.found} />

      {messaging.channels.length > 0 ? (
        <section aria-labelledby="channels-heading" className="flex flex-col gap-group">
          <h2
            id="channels-heading"
            className="text-sm font-semibold text-muted-foreground uppercase"
          >
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

// firstNonEmpty is the first value that is not the empty string, or '' when
// none is. It keeps the channel the preview posts to matching a listed option:
// falling through an unset configured channel to the first known channel rather
// than leaving the state empty while the select shows an option it never chose.
function firstNonEmpty(...values: string[]): string {
  for (const value of values) {
    if (value !== '') {
      return value
    }
  }

  return ''
}

// AnnounceSection offers to announce the branch's pull request, once there is
// one, and says where the announcement went — in a line that stays when the
// controls step aside.
function AnnounceSection({
  messaging,
  found,
}: {
  messaging: Snapshot['messaging']
  found: boolean
}) {
  const { noun } = useForgeWords()
  const outcome = useOutcome()

  return (
    <section aria-labelledby="announce-heading" className="flex flex-col gap-group">
      <h2 id="announce-heading" className="text-sm font-semibold text-muted-foreground uppercase">
        Announce
      </h2>
      {found ? (
        <AnnounceControls
          service={messaging.service}
          channels={messaging.channels}
          defaultChannel={messaging.channel}
          onAnnounced={outcome.say}
        />
      ) : (
        <p className="text-sm text-muted-foreground">
          Open a {noun} first — there is nothing to announce yet.
        </p>
      )}
      <OutcomeLine said={outcome.said} />
    </section>
  )
}

interface AnnounceControlsProps {
  service: string
  channels: string[]
  defaultChannel: string
  onAnnounced: (said: string) => void
}

// AnnounceControls posts the pull request's announcement to the configured
// service behind a preview step: it fetches the composed message, shows it for
// confirmation, and posts only on confirm — announcing is outward and not undone.
// The preview and the post are two steps, each its own action; a refused post
// goes back to the button, with its reason. Once posted, the controls step
// aside, and where it went is said through onAnnounced. Focus goes to the
// preview as it opens, and back to the button when it closes unposted.
function AnnounceControls({
  service,
  channels,
  defaultChannel,
  onAnnounced,
}: AnnounceControlsProps) {
  const [channel, setChannel] = useState(() => firstNonEmpty(defaultChannel, channels[0] ?? ''))
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const preview = useAsyncAction(
    async () => {
      const composed = await previewAnnouncement()
      setChannel(firstNonEmpty(composed.channel, defaultChannel, channels[0] ?? ''))

      return composed.text
    },
    {
      fallback:
        'The announcement could not be composed. Try again, or run workflow announce from a terminal.',
    },
  )
  const post = useAsyncAction(() => announce(channel), {
    fallback: 'Nothing was announced. Try again, or run workflow announce from a terminal.',
    // A webhook has no channel of its own to name, so the service stands in.
    done: (posted) => `Announced to ${posted.channel === '' ? service : posted.channel}.`,
    onDone: onAnnounced,
  })

  if (post.state === 'done') {
    return null
  }

  if (preview.state === 'done' && post.state !== 'error') {
    return (
      <AnnouncePreview
        text={preview.result ?? ''}
        channel={channel}
        channels={channels}
        posting={post.state === 'running'}
        onChannel={setChannel}
        onCancel={() => {
          handBack()
          preview.reset()
        }}
        onPost={() => {
          // Back to the button if the post is refused; a posted announcement
          // hands focus to the line that says where it went instead.
          handBack()
          void post.run()
        }}
      />
    )
  }

  const failure = preview.state === 'error' ? preview.error : post.error

  return (
    <div className="flex flex-col gap-tight">
      <button
        ref={opener}
        type="button"
        disabled={preview.state === 'running'}
        onClick={() => {
          post.reset()
          void preview.run()
        }}
        className="self-start rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
      >
        {preview.state === 'running' ? 'Preparing…' : `Announce to ${service}`}
      </button>
      {failure === '' ? null : (
        <p role="alert" className="text-sm text-destructive">
          {failure}
        </p>
      )}
    </div>
  )
}

// AnnouncePreview shows the composed message and the channel it will go to,
// with a confirm — named apart from the button that opened the preview, since
// only this one sends — and a cancel. It takes focus as it opens, so what is about to
// be sent is what a screen reader reads next.
function AnnouncePreview({
  text,
  channel,
  channels,
  posting,
  onChannel,
  onCancel,
  onPost,
}: {
  text: string
  channel: string
  channels: string[]
  posting: boolean
  onChannel: (channel: string) => void
  onCancel: () => void
  onPost: () => void
}) {
  const shown = useFocusOnMount<HTMLDivElement>()

  return (
    <div
      ref={shown}
      role="group"
      aria-label="Announcement preview"
      tabIndex={-1}
      className="flex flex-col gap-group rounded-lg border border-border p-4"
    >
      <pre className="rounded-md bg-muted p-3 font-sans text-sm whitespace-pre-wrap">{text}</pre>
      {channels.length > 0 ? (
        <label className="flex items-center gap-item text-sm">
          <span className="text-muted-foreground">Channel</span>
          <select
            value={channel}
            disabled={posting}
            onChange={(event) => {
              onChannel(event.target.value)
            }}
            className="rounded-md border border-input bg-transparent px-2 py-1 text-sm disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
          >
            {channels.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
      ) : null}
      <div className="flex items-center gap-item">
        <button
          type="button"
          disabled={posting}
          onClick={onCancel}
          className="rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
        >
          Cancel
        </button>
        <button
          type="button"
          disabled={posting}
          onClick={onPost}
          className="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
        >
          {posting ? 'Announcing…' : 'Announce now'}
        </button>
      </div>
    </div>
  )
}
