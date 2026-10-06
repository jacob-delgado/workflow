import { useState } from 'react'
import type { Activity, MessagingDestination, PostLength } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { ChannelSelect, PreviewText } from '@/features/messaging/AnnouncePreview.tsx'
import { postSummary } from './summaryApi.ts'
import { useHoldShortcuts, useShortcut } from '@/features/keyboard/useShortcut.ts'

// webhookChannel is where a post goes that names no channel: a webhook's own.
const webhookChannel = 'the channel its webhook is bound to'

// SummaryPost offers the summary to post to the messaging service, once one is
// set up: Post… opens a preview of the Markdown and where it goes, shaped as
// the Messaging section's announcement preview is, with Edit, the channel when
// there is a choice, Cancel, and Post, which alone sends. Nothing is posted
// before Post. A refused post keeps the preview, with why; a post that goes
// closes it and says where it went through teller.
export function SummaryPost({ activity, teller }: { activity: Activity; teller: Teller }) {
  const messaging = useSnapshotStore((state) => state.snapshot?.messaging)
  const [previewing, setPreviewing] = useState(false)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const postKeys = useShortcut('post-summary', opener)

  if (messaging === undefined || !messaging.configured) {
    return null
  }

  if (previewing) {
    return (
      <PostPreview
        activity={activity}
        messaging={messaging}
        teller={teller}
        onClose={(posted) => {
          if (!posted) {
            handBack()
          }
          setPreviewing(false)
        }}
      />
    )
  }

  return (
    <Button
      variant="secondary"
      ref={opener}
      aria-keyshortcuts={postKeys}
      onClick={() => {
        setPreviewing(true)
      }}
    >
      Post…
    </Button>
  )
}

interface PostPreviewProps {
  activity: Activity
  messaging: MessagingDestination
  teller: Teller
  onClose: (posted: boolean) => void
}

// PostPreview is the summary about to be posted and where, taking focus as it
// opens, so what is about to be sent is what a screen reader reads next.
function PostPreview({ activity, messaging, teller, onClose }: PostPreviewProps) {
  const shown = useFocusOnMount<HTMLDivElement>()
  useHoldShortcuts()
  const [channel, setChannel] = useState(
    messaging.channel === '' ? (messaging.channels[0] ?? '') : messaging.channel,
  )
  const [edited, setEdited] = useState<string | null>(null)
  const post = useAsyncAction(
    (text: string) => postSummary(activity.from, activity.to, text, channel),
    {
      fallback: 'Nothing was posted. Try again, or run workflow summary --post from a terminal.',
      done: (posted) => `Posted to ${posted.destination}.`,
      onStart: teller.clear,
      onDone: (said) => {
        teller.say(said)
        onClose(true)
      },
    },
  )
  const busy = post.state === 'running'

  return (
    <div
      ref={shown}
      role="group"
      aria-label="Summary preview"
      tabIndex={-1}
      className="flex w-full flex-col gap-group rounded-lg border border-border p-4"
    >
      <PreviewText
        label="Summary text"
        text={edited ?? activity.text}
        editing={edited !== null}
        busy={busy}
        onEdit={setEdited}
      />
      {activity.post_length === undefined ? null : (
        <PostLengthLine length={activity.post_length} edited={edited !== null} />
      )}
      {messaging.channels.length > 0 ? (
        <ChannelSelect
          channel={channel}
          channels={messaging.channels}
          busy={busy}
          onChannel={setChannel}
        />
      ) : (
        <p className="text-sm text-muted-foreground">
          To {channel === '' ? webhookChannel : channel}
        </p>
      )}
      {post.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {post.error}
        </p>
      ) : null}
      <div className="flex flex-wrap items-center gap-item">
        <Button
          variant="secondary"
          disabled={busy}
          onClick={() => {
            onClose(false)
          }}
        >
          Cancel
        </Button>
        <Button
          variant="primary"
          disabled={busy}
          onClick={() => {
            void post.run(edited ?? activity.text)
          }}
        >
          {busy ? 'Posting…' : 'Post'}
        </Button>
      </div>
    </div>
  )
}

// counted writes a number as the page writes counts, with its thousands
// marked.
function counted(count: number): string {
  return count.toLocaleString('en-US')
}

// PostLengthLine says how long the summary is, rendered for the service,
// against what the service takes — a warning when it is longer, since the
// post would be refused — or, once edited, only the limit, which the server
// measures the edit against as it is posted.
function PostLengthLine({ length, edited }: { length: PostLength; edited: boolean }) {
  const limit = `${counted(length.limit)} ${length.unit}`
  if (edited) {
    return (
      <p className="text-sm text-muted-foreground">
        {length.service} takes {limit} at most; the edit is measured as it is posted.
      </p>
    )
  }

  if (length.count > length.limit) {
    return (
      <p className="text-sm text-destructive">
        Too long for {length.service}: {counted(length.count)} of {limit}. Pick a shorter period, or
        edit it down.
      </p>
    )
  }

  return (
    <p className="text-sm text-muted-foreground">
      {counted(length.count)} of {limit} {length.service} takes
    </p>
  )
}
