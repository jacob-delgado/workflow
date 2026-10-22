import { useState } from 'react'
import type { AnnounceMentions } from '@/api/generated/types.gen.ts'
import { problemCode } from '@/api/apiError.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import { Failure } from '@/lib/Status.tsx'
import { type AsyncState, useAsyncAction } from '@/lib/useAsyncAction.ts'
import { announce, announceWhenCIPasses, previewAnnouncement } from './announceApi.ts'
import { AnnouncePreview } from './AnnouncePreview.tsx'
import { mentionsOf, pickFrom, type TagPick } from './tagPick.ts'
import { withWarning } from './withWarning.ts'

interface AnnounceControlsProps {
  // pullName is the pull request announced, by its sigil and number.
  pullName: string
  service: string
  channels: string[]
  defaultChannel: string
  announcing: Announcing
  // waiting is a held announcement waiting for CI, which announcing now
  // would take the place of.
  waiting: boolean
}

// Announcing is what a send from the section is handed: start as it goes,
// refuse when it was refused, and say what became of it.
export interface Announcing {
  start: () => void
  refuse: () => void
  say: (said: string) => void
}

// AnnounceControls posts the pull request's announcement to the configured
// service behind a preview step: it fetches the composed message, shows it for
// confirmation, and posts only on confirm, and only the text it showed — or
// the text it was edited to there — since announcing is outward and not
// undone. An announcement that can wait for CI is offered to be held until
// the CI passes, as the terminal's w does.
// The preview and the post are two steps, each its own action; a refused post
// goes back to the button, with its reason. Once posted or held, the controls
// step aside, and what became of it is said through announcing. Focus goes to
// the preview as it opens, and back to the button when it closes unposted.
// While a held announcement waits, the button is Announce now instead, a
// control beside Stop waiting rather than the section's act: the same preview,
// whose post takes the place of the waiting one.
export function AnnounceControls({
  pullName,
  service,
  channels,
  defaultChannel,
  announcing,
  waiting,
}: AnnounceControlsProps) {
  const draft = useDraft(defaultChannel, channels)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const postKeys = useShortcut('post', opener)
  // A check failed between the preview and the post: preview it again, which
  // offers to announce it once CI passes, and says why.
  const post = useAnnouncePost(draft.channel, service, announcing, () => {
    void draft.preview.run()
  })
  const hold = useAnnounceHold(draft.channel, service, announcing)

  if (post.state === 'done' || hold.state === 'done') {
    return null
  }

  const refused = refusedSend(post, hold)
  if (draft.preview.state === 'done' && !refused) {
    return (
      <PreviewStep draft={draft} post={post} hold={hold} pullName={pullName} onBack={handBack} />
    )
  }

  const failure = draft.preview.state === 'error' ? draft.preview.error : post.error || hold.error

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant={waiting ? 'secondary' : 'primary'}
        ref={opener}
        aria-keyshortcuts={postKeys}
        held={draft.preview.state === 'running'}
        onClick={() => {
          post.reset()
          hold.reset()
          void draft.preview.run()
        }}
        className="self-start"
      >
        {draft.preview.state === 'running' ? 'Preparing…' : announceLabel(service, waiting)}
      </Button>
      {failure === '' ? null : <Failure>{failure}</Failure>}
    </div>
  )
}

// announceLabel names the announce action: Announce now instead while a held
// announcement waits, since the post takes its place, and Announce to the
// service otherwise.
function announceLabel(service: string, waiting: boolean): string {
  return waiting ? 'Announce now instead' : `Announce to ${service}`
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
  pullName,
  onBack,
}: {
  draft: Draft
  post: Send
  hold: Send
  pullName: string
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
      ciFailedOn={draft.preview.result?.ci === 'failed' ? pullName : undefined}
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

// refusedSend reports a send the server refused, which goes back to the button
// with why — but for a post refused because a check failed, which is previewed
// again to offer the wait.
function refusedSend(
  post: { state: AsyncState; code: string },
  hold: { state: AsyncState },
): boolean {
  return (post.state === 'error' && post.code !== 'ci_failed') || hold.state === 'error'
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
// says where it went through announcing, and that it may be offered again
// when the store could not remember it. Only an announcement that tags asks
// for mentions, and only an edited one carries its edit, so one that does
// neither posts as it always has.
function useAnnouncePost(
  channel: string,
  service: string,
  announcing: Announcing,
  onCIFailed: () => void,
) {
  const send = (previewed: string, mentions?: AnnounceMentions, edited?: string) => {
    if (edited !== undefined) {
      return announce(channel, previewed, mentions, edited)
    }

    return mentions === undefined
      ? announce(channel, previewed)
      : announce(channel, previewed, mentions)
  }

  return useAsyncAction(
    async (previewed: string, mentions?: AnnounceMentions, edited?: string) => {
      try {
        return await send(previewed, mentions, edited)
      } catch (caught) {
        announcing.refuse()
        if (problemCode(caught) === 'ci_failed') {
          onCIFailed()
        }

        throw caught
      }
    },
    {
      fallback: 'Nothing was announced. Try again, or run workflow announce from a terminal.',
      // A webhook has no channel of its own to name, so the service stands in.
      done: (posted) =>
        withWarning(
          `Announced to ${posted.channel === '' ? service : posted.channel}.`,
          posted.warning,
        ),
      onStart: announcing.start,
      onDone: announcing.say,
    },
  )
}

// useAnnounceHold holds a previewed announcement to channel until the pull
// request's CI passes, and says so through announcing — or that it went at
// once, when the CI had passed by then, as a post says it.
function useAnnounceHold(channel: string, service: string, announcing: Announcing) {
  return useAsyncAction(
    async (previewed: string, mentions?: AnnounceMentions, edited?: string) => {
      try {
        return await announceWhenCIPasses(channel, previewed, mentions, edited)
      } catch (caught) {
        announcing.refuse()
        throw caught
      }
    },
    {
      fallback: 'Nothing was held. Try again, or announce it now.',
      done: (answer) => {
        const where = answer.channel === '' ? service : answer.channel

        return answer.held
          ? `Will announce to ${where} once CI passes.`
          : withWarning(`Announced to ${where}.`, answer.warning)
      },
      onStart: announcing.start,
      onDone: announcing.say,
    },
  )
}
