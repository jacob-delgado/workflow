import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { announce, previewAnnouncement } from './announceApi.ts'

export function SlackPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  if (!snapshot) {
    return <EmptyState>Connecting…</EmptyState>
  }

  const { slack, review } = snapshot

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

      <section aria-labelledby="announce-heading" className="flex flex-col gap-2">
        <h2 id="announce-heading" className="text-sm font-semibold text-muted-foreground uppercase">
          Announce
        </h2>
        {review.found ? (
          <AnnounceControls channels={slack.channels} defaultChannel={slack.channel} />
        ) : (
          <p className="text-sm text-muted-foreground">
            Open a pull request first — there is nothing to announce yet.
          </p>
        )}
      </section>

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

type AnnounceState = 'idle' | 'loading' | 'preview' | 'posting' | 'done' | 'error'

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

// AnnounceControls posts the pull request's announcement to Slack behind a
// preview step: it fetches the composed message, shows it for confirmation, and
// posts only on confirm — announcing is outward and not undone.
function AnnounceControls({
  channels,
  defaultChannel,
}: {
  channels: string[]
  defaultChannel: string
}) {
  const [state, setState] = useState<AnnounceState>('idle')
  const [text, setText] = useState('')
  const [channel, setChannel] = useState(() => firstNonEmpty(defaultChannel, channels[0] ?? ''))
  const [error, setError] = useState('')

  const openPreview = async () => {
    setState('loading')
    try {
      const preview = await previewAnnouncement()
      setText(preview.text)
      setChannel(firstNonEmpty(preview.channel, defaultChannel, channels[0] ?? ''))
      setError('')
      setState('preview')
    } catch (caught) {
      setError(apiErrorMessage(caught, 'The announcement could not be previewed.'))
      setState('error')
    }
  }

  const post = async () => {
    setState('posting')
    try {
      await announce(channel)
      setError('')
      setState('done')
    } catch (caught) {
      setError(apiErrorMessage(caught, 'The announcement could not be posted.'))
      setState('error')
    }
  }

  if (state === 'done') {
    return (
      <p className="text-sm text-success">Announced{channel === '' ? '' : ` to ${channel}`}.</p>
    )
  }

  if (state === 'preview' || state === 'posting') {
    return (
      <AnnouncePreview
        text={text}
        channel={channel}
        channels={channels}
        posting={state === 'posting'}
        onChannel={setChannel}
        onCancel={() => {
          setState('idle')
        }}
        onPost={() => {
          void post()
        }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-1">
      <button
        type="button"
        disabled={state === 'loading'}
        onClick={() => {
          void openPreview()
        }}
        className="self-start rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
      >
        {state === 'loading' ? 'Preparing…' : 'Announce to Slack'}
      </button>
      {state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}

// AnnouncePreview shows the composed message and the channel it will post to,
// with a confirm and a cancel.
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
  return (
    <div className="flex flex-col gap-3 rounded-md border border-border p-4">
      <pre className="rounded-md bg-muted p-3 text-sm whitespace-pre-wrap">{text}</pre>
      {channels.length > 0 ? (
        <label className="flex items-center gap-2 text-sm">
          <span className="text-muted-foreground">Channel</span>
          <select
            value={channel}
            disabled={posting}
            onChange={(event) => {
              onChannel(event.target.value)
            }}
            className="rounded-md border border-input bg-transparent px-2 py-1 text-sm disabled:opacity-60"
          >
            {channels.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
      ) : null}
      <div className="flex items-center gap-2">
        <button
          type="button"
          disabled={posting}
          onClick={onCancel}
          className="rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
        >
          Cancel
        </button>
        <button
          type="button"
          disabled={posting}
          onClick={onPost}
          className="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
        >
          {posting ? 'Posting…' : 'Post to Slack'}
        </button>
      </div>
    </div>
  )
}
