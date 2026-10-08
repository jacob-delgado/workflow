import { Bold, Code, Italic, Link, List, type LucideIcon } from 'lucide-react'
import { useId, useLayoutEffect, useRef, useState, type KeyboardEvent, type RefObject } from 'react'
import type { IssueDetail } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { useConfigRead } from '@/features/settings/configApi.ts'
import { Button } from '@/lib/Button.tsx'
import { FieldFrame } from '@/lib/Field.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { Reading } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, plural } from '@/lib/utils.ts'
import { usePostComment } from './commentApi.ts'
import { shownKey } from './issuePlaces.ts'
import { applyFormat, type Edit, type Format } from './wiki/markdownFormat.ts'
import { CommentBody } from './wiki/WikiText.tsx'

type Tab = 'write' | 'preview'

interface CommentComposerProps {
  issueKey: string
  tracker: IssueDetail['tracker']
}

// CommentComposer writes a comment on an issue under its thread. A forge
// issue's comment is always Markdown, which the forge renders; a Jira issue's
// is Markdown with jira.markdown_comments on, and the server turns it into
// wiki markup, or is otherwise posted as typed for Jira to read as wiki
// markup. Markdown brings a toolbar that writes its marks and a Preview. On
// GitLab the box says a line starting with / runs as a quick action. A posted
// comment clears the box and keeps focus in it, for the next; a refused one
// stays, beside why. A Jira issue's box waits for the configuration that says
// which, behind the shared Reading status, so it is drawn once, in its settled
// shape, rather than growing its bar when the read lands; a read that fails
// leaves it wiki markup, as Jira reads a comment by default.
export function CommentComposer({ issueKey, tracker }: CommentComposerProps) {
  const config = useConfigRead()

  if (tracker !== 'forge' && config.isPending) {
    return <Reading>Reading how comments are written…</Reading>
  }

  return (
    <Composer
      issueKey={issueKey}
      tracker={tracker}
      jiraMarkdown={config.data?.config.jira.markdown_comments === true}
    />
  )
}

// Composer is the comment box in its settled shape: Markdown, with its bar,
// or wiki markup.
function Composer({
  issueKey,
  tracker,
  jiraMarkdown,
}: CommentComposerProps & { jiraMarkdown: boolean }) {
  const onGitLab = useHealthStore((state) => state.health?.forge_noun === gitLabNoun)
  const forgeIssue = tracker === 'forge'
  const markdown = forgeIssue || jiraMarkdown
  const shown = shownKey({ key: issueKey, tracker })
  const [text, setText] = useState('')
  const [tab, setTab] = useState<Tab>('write')
  const box = useRef<HTMLTextAreaElement>(null)
  const shortcut = useShortcut('comment', box, 'focus')
  const ids = useComposerIds()
  const outcome = useOutcome()
  const post = useAsyncAction(usePostComment(issueKey), {
    fallback: `The comment could not be posted on ${shown}.`,
    done: () => `Commented on ${shown}.`,
    onStart: outcome.clear,
    onDone: (said) => {
      outcome.say(said)
      setText('')
      setTab('write')
      box.current?.focus()
    },
  })
  const busy = post.state === 'running'
  const restoreSelection = useSelection(box)

  const { blank, setBlank, send } = useSend(text, busy, post.run)

  // data-typing keeps every single-key shortcut off while the composer, or its
  // bar, has the focus: a key pressed here is writing.
  return (
    <div data-typing className="mt-item flex flex-col gap-item">
      <FieldFrame>
        {markdown ? (
          <ComposerBar
            ids={ids}
            tab={tab}
            onTab={setTab}
            onFormat={(format) => {
              const edit = formatted(box.current, format)
              if (edit !== null) {
                setText(edit.text)
                restoreSelection(edit)
              }
            }}
          />
        ) : null}
        <WritePanel
          ids={ids}
          issueKey={shown}
          markdown={markdown}
          quickActions={forgeIssue && onGitLab}
          shown={!markdown || tab === 'write'}
          box={box}
          shortcut={shortcut}
          text={text}
          busy={busy}
          onText={(typed) => {
            setText(typed)
            setBlank(false)
          }}
        />
        {markdown && tab === 'preview' ? <PreviewPanel ids={ids} text={text} /> : null}
        <ComposerFooter
          ids={ids}
          hint={hintFor(forgeIssue, markdown)}
          quickActions={forgeIssue && onGitLab}
          length={text.length}
          busy={busy}
          onSend={send}
        />
      </FieldFrame>
      <ComposerRefusal blank={blank} error={post.state === 'error' ? post.error : ''} />
      <OutcomeLine said={outcome.said} />
    </div>
  )
}

interface WritePanelProps {
  ids: Ids
  issueKey: string
  markdown: boolean
  quickActions: boolean
  shown: boolean
  box: RefObject<HTMLTextAreaElement | null>
  // shortcut is the box's aria-keyshortcuts, the keys that put the focus in it.
  shortcut: string | undefined
  text: string
  busy: boolean
  onText: (text: string) => void
}

// WritePanel is the box a comment is written in: Write's panel when the
// comment is Markdown, and the whole composer's body otherwise. It is kept
// while Preview shows, so the selection and the undo history stay.
function WritePanel({
  ids,
  issueKey,
  markdown,
  quickActions,
  shown,
  box,
  shortcut,
  text,
  busy,
  onText,
}: WritePanelProps) {
  return (
    <div
      {...(markdown
        ? { role: 'tabpanel', id: ids.writePanel, 'aria-labelledby': ids.writeTab }
        : {})}
      hidden={!shown}
    >
      <label htmlFor={ids.box} className="sr-only">
        Comment on {issueKey}
      </label>
      <textarea
        ref={box}
        id={ids.box}
        aria-keyshortcuts={shortcut}
        value={text}
        readOnly={busy}
        rows={4}
        placeholder={markdown ? 'Write a comment in Markdown…' : 'Write a comment…'}
        aria-describedby={
          quickActions ? `${ids.hint} ${ids.quick} ${ids.count}` : `${ids.hint} ${ids.count}`
        }
        onChange={(event) => {
          onText(event.target.value)
        }}
        className="block min-h-24 w-full resize-y bg-transparent px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus-visible:outline-none"
      />
    </div>
  )
}

// ComposerRefusal says why a comment was not posted: there was nothing to
// post, or the server refused it.
function ComposerRefusal({ blank, error }: { blank: boolean; error: string }) {
  const reason = blank ? 'Write a comment first.' : error

  return reason === '' ? null : (
    <p role="alert" className="text-sm text-destructive">
      {reason}
    </p>
  )
}

type Ids = ReturnType<typeof useComposerIds>

// useComposerIds names the composer's parts, so its tabs, panels, hint and
// count can refer to one another.
function useComposerIds() {
  const id = useId()

  return {
    box: `${id}-box`,
    writeTab: `${id}-write-tab`,
    previewTab: `${id}-preview-tab`,
    writePanel: `${id}-write`,
    previewPanel: `${id}-preview`,
    hint: `${id}-hint`,
    quick: `${id}-quick`,
    count: `${id}-count`,
  }
}

// formatted is the box's text with format applied over its selection, or
// null when there is no box.
function formatted(box: HTMLTextAreaElement | null, format: Format): Edit | null {
  return box === null ? null : applyFormat(format, box.value, box.selectionStart, box.selectionEnd)
}

// useSelection returns a function that selects part of the box once the text
// an edit wrote is drawn, and puts focus back in it from the button pressed.
// The selection waits in a ref: it is for the DOM, not for drawing.
function useSelection(box: RefObject<HTMLTextAreaElement | null>): (edit: Edit) => void {
  const pending = useRef<Edit | null>(null)

  useLayoutEffect(() => {
    const edit = pending.current
    if (edit !== null && box.current?.value === edit.text) {
      box.current.focus()
      box.current.setSelectionRange(edit.start, edit.end)
      pending.current = null
    }
  })

  return (edit: Edit) => {
    pending.current = edit
  }
}

const tabs: { tab: Tab; label: string }[] = [
  { tab: 'write', label: 'Write' },
  { tab: 'preview', label: 'Preview' },
]

interface ComposerBarProps {
  ids: Ids
  tab: Tab
  onTab: (tab: Tab) => void
  onFormat: (format: Format) => void
}

// ComposerBar heads a Markdown comment: Write and Preview as tabs, which the
// arrow keys move between, and the formatting buttons while writing.
function ComposerBar({ ids, tab, onTab, onFormat }: ComposerBarProps) {
  const tabRefs = useRef<Record<Tab, HTMLButtonElement | null>>({ write: null, preview: null })

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
      return
    }

    event.preventDefault()
    const next: Tab = tab === 'write' ? 'preview' : 'write'
    onTab(next)
    tabRefs.current[next]?.focus()
  }

  return (
    <div className="flex flex-wrap items-center justify-between gap-item border-b border-border bg-muted/40 px-2 py-1.5">
      <div role="tablist" aria-label="Comment" className="flex gap-tight rounded-md bg-muted p-0.5">
        {/* Not a Button: a tab of the tab list, drawn as a tab. */}
        {tabs.map(({ tab: each, label }) => (
          <button
            key={each}
            ref={(element) => {
              tabRefs.current[each] = element
            }}
            type="button"
            role="tab"
            id={each === 'write' ? ids.writeTab : ids.previewTab}
            aria-selected={tab === each}
            aria-controls={each === 'write' ? ids.writePanel : ids.previewPanel}
            tabIndex={tab === each ? 0 : -1}
            onClick={() => {
              onTab(each)
            }}
            onKeyDown={onKeyDown}
            className={cn(
              'rounded-sm px-2.5 py-1 text-xs font-medium focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
              tab === each
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === 'write' ? <FormatButtons onFormat={onFormat} /> : null}
    </div>
  )
}

const formatButtons: { format: Format; label: string; Icon: LucideIcon }[] = [
  { format: 'bold', label: 'Bold', Icon: Bold },
  { format: 'italic', label: 'Italic', Icon: Italic },
  { format: 'code', label: 'Code', Icon: Code },
  { format: 'link', label: 'Link', Icon: Link },
  { format: 'list', label: 'Bulleted list', Icon: List },
]

// FormatButtons write Markdown's marks around the selection.
function FormatButtons({ onFormat }: { onFormat: (format: Format) => void }) {
  return (
    <div role="group" aria-label="Formatting" className="flex gap-tight">
      {/* Not a Button: a toolbar's icon, which keeps the selection in the box. */}
      {formatButtons.map(({ format, label, Icon }) => (
        <button
          key={format}
          type="button"
          aria-label={label}
          title={label}
          // Keep the selection in the box: a press that took focus first would
          // leave the selection where the button cannot read it in every browser.
          onMouseDown={(event) => {
            event.preventDefault()
          }}
          onClick={() => {
            onFormat(format)
          }}
          className="grid size-7 place-items-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          <Icon aria-hidden className="size-4" />
        </button>
      ))}
    </div>
  )
}

// PreviewPanel draws the comment as Jira will show it.
function PreviewPanel({ ids, text }: { ids: Ids; text: string }) {
  return (
    <div
      role="tabpanel"
      id={ids.previewPanel}
      aria-labelledby={ids.previewTab}
      // A panel with nothing to tab to is reached by Tab itself, as the tabs
      // pattern asks, so a keyboard reader can scroll a long preview.
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- the tabs pattern makes a panel with no focusable content a tab stop (WAI-ARIA APG)
      tabIndex={0}
      className="min-h-24 px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset"
    >
      {text.trim() === '' ? (
        <p className="text-muted-foreground">Nothing to preview yet.</p>
      ) : (
        <CommentBody body={text} markdown={true} />
      )}
    </div>
  )
}

interface ComposerFooterProps {
  ids: Ids
  hint: string
  quickActions: boolean
  length: number
  busy: boolean
  onSend: () => void
}

// gitLabNoun is what the server's health calls a proposed change on GitLab,
// the one forge whose comments run slash lines as quick actions.
const gitLabNoun = 'merge request'

// hintFor says how a comment is read: by the forge, as Markdown, or by Jira,
// as Markdown converted or as wiki markup.
function hintFor(forgeIssue: boolean, markdown: boolean): string {
  if (forgeIssue) {
    return 'Markdown is supported; the forge renders it.'
  }

  return markdown ? 'Markdown is supported.' : 'Jira reads this as wiki markup.'
}

// ComposerFooter says how the comment is read and how long it is, beside the
// button that posts it.
function ComposerFooter({ ids, hint, quickActions, length, busy, onSend }: ComposerFooterProps) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-item border-t border-border px-3 py-2">
      <p className="flex flex-wrap gap-x-group text-xs text-muted-foreground">
        <span id={ids.hint}>{hint}</span>
        {quickActions ? (
          <span id={ids.quick}>A line starting with / runs as a GitLab quick action.</span>
        ) : null}
        <span id={ids.count} className="tabular-nums">
          {plural(length, 'character')}
        </span>
      </p>
      <Button variant="primary" held={busy} onClick={onSend}>
        {busy ? 'Commenting…' : 'Comment'}
      </Button>
    </div>
  )
}

// useSend sends what is typed unless a send is in flight, and refuses a blank
// comment, saying so until the next keystroke.
function useSend(text: string, busy: boolean, run: (text: string) => Promise<void>) {
  const [blank, setBlank] = useState(false)

  const send = () => {
    if (busy) {
      return
    }

    setBlank(text.trim() === '')
    if (text.trim() !== '') {
      void run(text)
    }
  }

  return { blank, setBlank, send }
}
