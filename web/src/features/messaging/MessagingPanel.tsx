import { useState } from 'react'
import type { AnnounceMentions, Snapshot } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { useLiveSnapshot } from '@/api/snapshot.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { type AsyncState, useAsyncAction } from '@/lib/useAsyncAction.ts'
import { definitionList } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { announce, announceWhenCIPasses, previewAnnouncement } from './announceApi.ts'
import { AnnouncePreview } from './AnnouncePreview.tsx'
import { HeldAnnouncement } from './HeldAnnouncement.tsx'
import { mentionsOf, pickFrom, type TagPick } from './tagPick.ts'

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
    <div className="flex max-w-2xl flex-col gap-section">
      <dl className={definitionList}>
        <dt className="text-muted-foreground">Service</dt>
        <dd>{messaging.service}</dd>
        <dt className="text-muted-foreground">Channel</dt>
        <dd>{messaging.channel === '' ? '—' : messaging.channel}</dd>
        <dt className="text-muted-foreground">Announcing as</dt>
        <dd>{messaging.author === '' ? 'the webhook' : messaging.author}</dd>
      </dl>

      <AnnounceSection
        messaging={messaging}
        found={review.found}
        held={snapshot.queued_announcement}
      />

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
  held,
}: {
  messaging: Snapshot['messaging']
  found: boolean
  held: Snapshot['queued_announcement']
}) {
  const { noun } = useForgeWords()
  const outcome = useOutcome()

  return (
    <section aria-labelledby="announce-heading" className="flex flex-col gap-group">
      <h2 id="announce-heading" className="text-base font-semibold">
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
      <HeldAnnouncement held={held} service={messaging.service} onSaid={outcome.say} />
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
// confirmation, and posts only on confirm, and only the text it showed — or
// the text it was edited to there — since announcing is outward and not
// undone. An announcement that can wait for CI is offered to be held until
// the CI passes, as the terminal's w does.
// The preview and the post are two steps, each its own action; a refused post
// goes back to the button, with its reason. Once posted or held, the controls
// step aside, and what became of it is said through onAnnounced. Focus goes to
// the preview as it opens, and back to the button when it closes unposted.
function AnnounceControls({
  service,
  channels,
  defaultChannel,
  onAnnounced,
}: AnnounceControlsProps) {
  const draft = useDraft(defaultChannel, channels)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const post = useAnnouncePost(draft.channel, service, onAnnounced)
  const hold = useAnnounceHold(draft.channel, service, onAnnounced)

  if (post.state === 'done' || hold.state === 'done') {
    return null
  }

  const refused = post.state === 'error' || hold.state === 'error'
  if (draft.preview.state === 'done' && !refused) {
    return <PreviewStep draft={draft} post={post} hold={hold} onBack={handBack} />
  }

  const failure = draft.preview.state === 'error' ? draft.preview.error : post.error || hold.error

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="primary"
        ref={opener}
        disabled={draft.preview.state === 'running'}
        onClick={() => {
          post.reset()
          hold.reset()
          void draft.preview.run()
        }}
        className="self-start"
      >
        {draft.preview.state === 'running' ? 'Preparing…' : `Announce to ${service}`}
      </Button>
      {failure === '' ? null : (
        <p role="alert" className="text-sm text-destructive">
          {failure}
        </p>
      )}
    </div>
  )
}

// useDraft is the announcement being prepared: its preview, read for the
// channel it opens on — so a scope that channel needs is the one it names —
// the channel it goes to, whom it tags, whether a link is being saved, and the
// text it was edited to, or null while it is not edited.
function useDraft(defaultChannel: string, channels: string[]) {
  const [channel, setChannel] = useState(() => firstNonEmpty(defaultChannel, channels[0] ?? ''))
  const [pick, setPick] = useState<TagPick>(() => pickFrom(undefined))
  const [linking, setLinking] = useState(false)
  const [edited, setEdited] = useState<string | null>(null)
  const preview = useAsyncAction(
    async () => {
      const composed = await previewAnnouncement(channel)
      setChannel(firstNonEmpty(composed.channel, defaultChannel, channels[0] ?? ''))
      setPick(pickFrom(composed.tagging))
      setEdited(null)

      return composed
    },
    {
      fallback:
        'The announcement could not be composed. Try again, or run workflow announce from a terminal.',
    },
  )

  return {
    channels,
    channel,
    setChannel,
    pick,
    setPick,
    linking,
    setLinking,
    edited,
    setEdited,
    preview,
  }
}

type Draft = ReturnType<typeof useDraft>
// Send is a send of the preview: a post now, or a hold for CI.
interface Send {
  state: AsyncState
  run: (previewed: string, mentions?: AnnounceMentions, edited?: string) => Promise<void>
}

// PreviewStep is the preview of the drafted announcement, wired to its sends:
// each posts the text shown — the composed one, which the server checks — with
// its edit and its mentions, when it has them.
function PreviewStep({
  draft,
  post,
  hold,
  onBack,
}: {
  draft: Draft
  post: Send
  hold: Send
  onBack: () => void
}) {
  const shown = draft.preview.result?.text ?? ''
  const tagging = draft.preview.result?.tagging
  const send = (via: Send) => {
    // Back to the button if the post is refused; a posted announcement hands
    // focus to the line that says where it went instead.
    onBack()
    const mentions = tagging?.available === true ? mentionsOf(draft.pick) : undefined
    void via.run(shown, mentions, editOf(shown, draft.edited))
  }

  return (
    <AnnouncePreview
      text={draft.edited ?? shown}
      editing={draft.edited !== null}
      channel={draft.channel}
      channels={draft.channels}
      tagging={tagging}
      pick={draft.pick}
      sending={sendingOf(post.state, hold.state)}
      linking={draft.linking}
      canWait={draft.preview.result?.can_wait_for_ci === true}
      onEdit={draft.setEdited}
      onChannel={draft.setChannel}
      onPick={draft.setPick}
      onLinking={draft.setLinking}
      onCancel={() => {
        onBack()
        draft.preview.reset()
      }}
      onPost={() => {
        send(post)
      }}
      onHold={() => {
        send(hold)
      }}
    />
  )
}

// editOf is the edit to send: none when the text was not edited, or was
// edited back to what was shown.
function editOf(shown: string, edited: string | null): string | undefined {
  return edited === null || edited === shown ? undefined : edited
}

// sendingOf is which send is in flight: the post, the hold, or neither.
function sendingOf(post: AsyncState, hold: AsyncState): 'post' | 'hold' | null {
  if (post === 'running') {
    return 'post'
  }

  return hold === 'running' ? 'hold' : null
}

// useAnnouncePost is the post of a previewed announcement to channel, which
// says where it went through onAnnounced. Only an announcement that tags asks
// for mentions, and only an edited one carries its edit, so one that does
// neither posts as it always has.
function useAnnouncePost(channel: string, service: string, onAnnounced: (said: string) => void) {
  return useAsyncAction(
    (previewed: string, mentions?: AnnounceMentions, edited?: string) => {
      if (edited !== undefined) {
        return announce(channel, previewed, mentions, edited)
      }

      return mentions === undefined
        ? announce(channel, previewed)
        : announce(channel, previewed, mentions)
    },
    {
      fallback: 'Nothing was announced. Try again, or run workflow announce from a terminal.',
      // A webhook has no channel of its own to name, so the service stands in.
      done: (posted) => `Announced to ${posted.channel === '' ? service : posted.channel}.`,
      onDone: onAnnounced,
    },
  )
}

// useAnnounceHold holds a previewed announcement to channel until the pull
// request's CI passes, and says so through onAnnounced — or that it went at
// once, when the CI had passed by then.
function useAnnounceHold(channel: string, service: string, onAnnounced: (said: string) => void) {
  return useAsyncAction(
    (previewed: string, mentions?: AnnounceMentions, edited?: string) =>
      announceWhenCIPasses(channel, previewed, mentions, edited),
    {
      fallback: 'Nothing was held. Try again, or announce it now.',
      done: (answer) => {
        const where = answer.channel === '' ? service : answer.channel

        return answer.held ? `Will announce to ${where} once CI passes.` : `Announced to ${where}.`
      },
      onDone: onAnnounced,
    },
  )
}
