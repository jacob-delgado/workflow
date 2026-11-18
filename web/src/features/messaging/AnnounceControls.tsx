import { useState } from 'react'
import type { Announcement, AnnounceMentions } from '@/api/generated/types.gen.ts'
import { problemCode } from '@/api/apiError.ts'
import { useForgeWords } from '@/api/health.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import { Failure } from '@/lib/Status.tsx'
import { type Attempt, refusalOf, useAsyncAction } from '@/lib/useAsyncAction.ts'
import { announce, announceWhenCIPasses, type Held, previewAnnouncement } from './announceApi.ts'
import { AnnouncePreview } from './AnnouncePreview.tsx'
import { mentionsOf, pickFrom, type TagPick } from './tagPick.ts'
import { settled, settledSaying } from './settledSaying.ts'
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
// goes back to the button, with its reason. Once posted or held, the preview
// closes and the button is drawn again, and what became of it is said through
// announcing; the section steps the controls aside once a frame shows the
// pull request announced. Focus goes to the preview as it opens, and back to
// the button when it closes. While the frame shows a held announcement
// waiting, the button is Announce now instead, a control beside Stop waiting
// rather than the section's act: the same preview, whose post, once it goes,
// settles the waiting one as announced.
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
  // A send that went closes the preview, drawing the button again, and hands
  // focus to the line that says where it went; one refused hands it back to
  // the button.
  const sent: Announcing = {
    start: announcing.start,
    refuse: () => {
      handBack()
      announcing.refuse()
    },
    say: (said) => {
      draft.preview.reset()
      announcing.say(said)
    },
  }
  // A check failed between the preview and the post: preview it again, which
  // offers to announce it once CI passes, and says why, keeping the channel,
  // the tags and the edit chosen for the post.
  const post = useAnnouncePost(draft.channel, service, sent, () => {
    void draft.preview.run('keeping the choices')
  })
  // A hold lets go of a post refused for a failed check, whose reason the
  // hold answers: once the hold is made, it would stand under the button.
  const hold = useAnnounceHold(draft.channel, service, {
    ...sent,
    start: () => {
      post.reset()
      sent.start()
    },
  })

  const step = announceStep(draft.preview, post, hold)
  switch (step.step) {
    case 'previewing':
      return (
        <PreviewStep
          draft={draft}
          composed={step.composed}
          sending={step.sending}
          post={post}
          hold={hold}
          pullName={pullName}
          onBack={handBack}
        />
      )
    case 'offer':
    case 'preparing':
      return (
        <div className="flex flex-col gap-tight">
          <Button
            variant={waiting ? 'secondary' : 'primary'}
            ref={opener}
            aria-keyshortcuts={postKeys}
            held={step.step === 'preparing'}
            onClick={() => {
              post.reset()
              hold.reset()
              void draft.preview.run('afresh')
            }}
            className="self-start"
          >
            {step.step === 'preparing' ? 'Preparing…' : announceLabel(service, waiting)}
          </Button>
          {step.failure === '' ? null : <Failure>{step.failure}</Failure>}
        </div>
      )
  }
}

// AnnounceStep is where the controls stand, derived each render from the
// preview and the two sends, which stay the only sources: the button, with
// why the last try failed or ''; the button held while the preview reads,
// keeping the reason a post refused for a failed check gave; or the composed
// preview, with the send in flight, if any.
type AnnounceStep =
  | { step: 'offer'; failure: string }
  | { step: 'preparing'; failure: string }
  | { step: 'previewing'; composed: Announcement; sending: Sending }

// Sending is which send of the preview is in flight: the post, the hold, or
// neither.
type Sending = 'post' | 'hold' | null

// announceStep is where the controls stand, by the preview's state.
function announceStep(
  preview: Attempt<Announcement>,
  post: Attempt<unknown>,
  hold: Attempt<unknown>,
): AnnounceStep {
  switch (preview.state) {
    case 'idle':
      return { step: 'offer', failure: refusalOf(post, hold) }
    case 'running':
      return { step: 'preparing', failure: refusalOf(post, hold) }
    case 'error':
      return { step: 'offer', failure: preview.error }
    case 'done':
      return postStep(preview.result, post, hold)
  }
}

// postStep is where a composed preview stands by its post: a refused post
// goes back to the button with why — but for one refused because a check
// failed, which is previewed again to offer the wait, and stands as its hold
// does.
function postStep(
  composed: Announcement,
  post: Attempt<unknown>,
  hold: Attempt<unknown>,
): AnnounceStep {
  switch (post.state) {
    case 'running':
      return { step: 'previewing', composed, sending: 'post' }
    case 'error':
      return post.code === 'ci_failed'
        ? holdStep(composed, hold)
        : { step: 'offer', failure: post.error }
    case 'idle':
    case 'done':
      return holdStep(composed, hold)
  }
}

// holdStep is where a composed preview stands by its hold: a refused hold
// goes back to the button with why.
function holdStep(composed: Announcement, hold: Attempt<unknown>): AnnounceStep {
  switch (hold.state) {
    case 'running':
      return { step: 'previewing', composed, sending: 'hold' }
    case 'error':
      return { step: 'offer', failure: hold.error }
    case 'idle':
    case 'done':
      return { step: 'previewing', composed, sending: null }
  }
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

// Redraft is what a preview read again keeps of the draft: nothing, for a new
// one, or the channel, the tags and the edit chosen, for one previewed again
// because its post was refused.
type Redraft = 'afresh' | 'keeping the choices'

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
    async (redraft: Redraft) => {
      const composed = await previewAnnouncement(channel)
      if (redraft === 'afresh') {
        setChannel(firstNonEmpty(composed.channel, defaultChannel, channels[0] ?? ''))
        setPick(pickFrom(composed.tagging))
        setEdited(null)
      }

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
  run: (previewed: string, mentions?: AnnounceMentions, edited?: string) => Promise<void>
}

// PreviewStep is the preview of the drafted announcement, composed as the
// server answered it, wired to its sends: each posts the text shown — the
// composed one, which the server checks — with its edit and its mentions, when
// it has them.
function PreviewStep({
  draft,
  composed,
  sending,
  post,
  hold,
  pullName,
  onBack,
}: {
  draft: Draft
  composed: Announcement
  sending: Sending
  post: Send
  hold: Send
  pullName: string
  onBack: () => void
}) {
  const shown = composed.text
  const tagging = composed.tagging
  const send = (via: Send) => {
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
      sending={sending}
      linking={draft.linking}
      canWait={composed.can_wait_for_ci === true}
      ciFailedOn={composed.ci === 'failed' ? pullName : undefined}
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

// useAnnouncePost is the post of a previewed announcement to channel, which
// says where it went through announcing, and that it is not offered again
// when the store could not confirm it. Only an announcement that tags asks
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
// request's CI passes, and says so through announcing — and which pull
// request's waiting announcement it replaced, if it did — or that it went at
// once, when the CI had passed by then, as a post says it; or, held, that it
// no longer waits, as heldSaying words it.
function useAnnounceHold(channel: string, service: string, announcing: Announcing) {
  const { sigil } = useForgeWords()

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

        if (!answer.held) {
          return withWarning(`Announced to ${where}.`, answer.warning)
        }

        return heldSaying(answer, where, sigil)
      },
      onStart: announcing.start,
      onDone: announcing.say,
    },
  )
}

// heldSaying is what a hold the server answered says, by its state: one
// settled, stopped, dropped or posted elsewhere, as settledSaying words it
// wherever a hold is shown; and any other state, waiting or being posted,
// that it will announce once CI passes, and which pull request's waiting
// announcement it replaced, if it did.
function heldSaying(answer: Held, where: string, sigil: string): string {
  if (settled(answer.state)) {
    return settledSaying(answer.state, answer, where)
  }

  const replaced =
    answer.replaced === undefined
      ? ''
      : ` It replaced the announcement waiting for ${sigil}${String(answer.replaced)}.`

  return `Will announce to ${where} once CI passes.${replaced}`
}
