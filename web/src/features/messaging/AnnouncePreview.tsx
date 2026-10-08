import type { Dispatch, SetStateAction } from 'react'
import type { AnnouncementTagging } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { Select, TextArea } from '@/lib/Field.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { TagPicker } from './TagPicker.tsx'
import type { TagPick } from './tagPick.ts'
import { useHoldShortcuts } from '@/lib/LastLook.tsx'

interface AnnouncePreviewProps {
  text: string
  editing: boolean
  channel: string
  channels: string[]
  tagging: AnnouncementTagging | undefined
  pick: TagPick
  sending: 'post' | 'hold' | null
  linking: boolean
  canWait: boolean
  onEdit: (text: string) => void
  onChannel: (channel: string) => void
  onPick: Dispatch<SetStateAction<TagPick>>
  onLinking: (linking: boolean) => void
  onCancel: () => void
  onPost: () => void
  onHold: () => void
}

// AnnouncePreview shows the composed message — or, once Edit is pressed, a box
// holding it to edit, as the terminal's e opens it in an editor — the channel
// it will go to and, for an announcement that tags, whom it tags, with a
// confirm — named apart from the button that opened the preview, since only
// this one sends — and a cancel. One that can wait for CI also offers to be
// held until the CI passes. The sends wait while an owner's link is being
// saved, so the post tags whom the preview ends up showing. It takes focus as
// it opens, so what is about to be sent is what a screen reader reads next.
export function AnnouncePreview(props: AnnouncePreviewProps) {
  const { channel, channels, tagging, pick, sending, onChannel, onPick, onLinking } = props
  const shown = useFocusOnMount<HTMLDivElement>()
  const busy = sending !== null
  useHoldShortcuts()

  return (
    <div
      ref={shown}
      role="group"
      aria-label="Announcement preview"
      tabIndex={-1}
      className="flex flex-col gap-group rounded-lg border border-border p-4"
    >
      <PreviewText
        label="Announcement text"
        text={props.text}
        editing={props.editing}
        busy={busy}
        onEdit={props.onEdit}
      />
      {channels.length > 0 ? (
        <ChannelSelect channel={channel} channels={channels} busy={busy} onChannel={onChannel} />
      ) : null}
      <UntaggedNote reason={tagging?.unavailable_reason} />
      {tagging?.available === true ? (
        <TagPicker
          tagging={tagging}
          pick={pick}
          channel={channel}
          posting={busy}
          onPick={onPick}
          onLinking={onLinking}
        />
      ) : null}
      <PreviewActions {...props} />
    </div>
  )
}

// ChannelSelect is the channel a post goes to, chosen from those configured.
export function ChannelSelect({
  channel,
  channels,
  busy,
  onChannel,
}: {
  channel: string
  channels: string[]
  busy: boolean
  onChannel: (channel: string) => void
}) {
  return (
    <label className="flex items-center gap-item text-sm">
      <span className="text-muted-foreground">Channel</span>
      <Select
        size="sm"
        value={channel}
        held={busy}
        onChange={(event) => {
          onChannel(event.target.value)
        }}
      >
        {channels.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </Select>
    </label>
  )
}

// PreviewText is the message as it will be posted, with Edit, or the box,
// named label, it is being edited in. An edit emptied of text is refused by
// the server, which says so.
export function PreviewText({
  label,
  text,
  editing,
  busy,
  onEdit,
}: {
  label: string
  text: string
  editing: boolean
  busy: boolean
  onEdit: (text: string) => void
}) {
  if (editing) {
    return (
      <label className="flex flex-col gap-tight text-sm">
        <span className="font-medium">{label}</span>
        <TextArea
          rows={Math.min(12, text.split('\n').length + 2)}
          value={text}
          held={busy}
          onChange={(event) => {
            onEdit(event.target.value)
          }}
        />
      </label>
    )
  }

  return (
    <div className="flex flex-col items-start gap-item">
      <pre className="self-stretch rounded-md bg-muted p-3 font-sans text-sm whitespace-pre-wrap">
        {text}
      </pre>
      <Button
        variant="secondary"
        size="sm"
        held={busy}
        onClick={() => {
          onEdit(text)
        }}
      >
        Edit
      </Button>
    </div>
  )
}

// PreviewActions are the preview's Cancel and its sends: Announce now and,
// for an announcement that can wait for CI, Announce when CI passes.
function PreviewActions({
  sending,
  linking,
  canWait,
  onCancel,
  onPost,
  onHold,
}: AnnouncePreviewProps) {
  const busy = sending !== null

  return (
    <div className="flex flex-wrap items-center gap-item">
      <Button variant="secondary" held={busy} onClick={onCancel}>
        Cancel
      </Button>
      {canWait ? (
        <Button variant="secondary" held={busy || linking} onClick={onHold}>
          {sending === 'hold' ? 'Announcing when CI passes…' : 'Announce when CI passes'}
        </Button>
      ) : null}
      <Button variant="primary" held={busy || linking} onClick={onPost}>
        {sending === 'post' ? 'Announcing…' : 'Announce now'}
      </Button>
    </div>
  )
}

// UntaggedNote says why an announcement that would tag tags no one — the
// Slack workspace the token is for could not be read — when that is why.
function UntaggedNote({ reason }: { reason: string | undefined }) {
  if (reason === undefined) {
    return null
  }

  return (
    <p role="note" className="text-sm text-muted-foreground">
      {reason}. The announcement posts untagged.
    </p>
  )
}
