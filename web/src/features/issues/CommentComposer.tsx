import { Bold, Code, Italic, Link, List, type LucideIcon } from 'lucide-react'
import { useId, useLayoutEffect, useRef, useState, type KeyboardEvent, type RefObject } from 'react'
import { useConfigRead } from '@/features/settings/configApi.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn } from '@/lib/utils.ts'
import { usePostComment } from './commentApi.ts'
import { applyFormat, type Edit, type Format } from './wiki/markdownFormat.ts'
import { wikiFromMarkdown } from './wiki/wikiFromMarkdown.ts'
import { WikiText } from './wiki/WikiText.tsx'

type Tab = 'write' | 'preview'

// CommentComposer writes a comment on a Jira issue under its thread. With
// jira.markdown_comments on, the comment is Markdown: a toolbar writes its
// marks, and Preview draws it as Jira will once the server turns it into
// wiki markup. Otherwise it is posted as typed, and Jira reads it as wiki
// markup. A posted comment clears the box and keeps focus in it, for the
// next; a refused one stays, beside why.
export function CommentComposer({ issueKey }: { issueKey: string }) {
  const markdown = useConfigRead().data?.config.jira.markdown_comments === true
  const [text, setText] = useState('')
  const [tab, setTab] = useState<Tab>('write')
  const [blank, setBlank] = useState(false)
  const box = useRef<HTMLTextAreaElement>(null)
  const ids = useComposerIds()
  const outcome = useOutcome()
  const post = useAsyncAction(usePostComment(issueKey), {
    fallback: `The comment could not be posted on ${issueKey}.`,
    done: () => `Commented on ${issueKey}.`,
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

  const send = () => {
    if (busy) {
      return
    }

    setBlank(text.trim() === '')
    if (text.trim() !== '') {
      void post.run(text)
    }
  }

  return (
    <div className="mt-item flex flex-col gap-item">
      <div className="flex flex-col overflow-hidden rounded-lg border border-input bg-card focus-within:border-ring focus-within:ring-1 focus-within:ring-ring">
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
          issueKey={issueKey}
          markdown={markdown}
          shown={!markdown || tab === 'write'}
          box={box}
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
          markdown={markdown}
          length={text.length}
          busy={busy}
          onSend={send}
        />
      </div>
      <ComposerRefusal blank={blank} error={post.state === 'error' ? post.error : ''} />
      <OutcomeLine said={outcome.said} />
    </div>
  )
}

interface WritePanelProps {
  ids: Ids
  issueKey: string
  markdown: boolean
  shown: boolean
  box: RefObject<HTMLTextAreaElement | null>
  text: string
  busy: boolean
  onText: (text: string) => void
}

// WritePanel is the box a comment is written in: Write's panel when the
// comment is Markdown, and the whole composer's body otherwise. It is kept
// while Preview shows, so the selection and the undo history stay.
function WritePanel({ ids, issueKey, markdown, shown, box, text, busy, onText }: WritePanelProps) {
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
        value={text}
        readOnly={busy}
        rows={4}
        placeholder={markdown ? 'Write a comment in Markdown…' : 'Write a comment…'}
        aria-describedby={`${ids.hint} ${ids.count}`}
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
        <WikiText markup={wikiFromMarkdown(text)} />
      )}
    </div>
  )
}

interface ComposerFooterProps {
  ids: Ids
  markdown: boolean
  length: number
  busy: boolean
  onSend: () => void
}

// ComposerFooter says how the comment is read and how long it is, beside the
// button that posts it.
function ComposerFooter({ ids, markdown, length, busy, onSend }: ComposerFooterProps) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-item border-t border-border px-3 py-2">
      <p className="flex flex-wrap gap-x-group text-xs text-muted-foreground">
        <span id={ids.hint}>
          {markdown ? 'Markdown is supported.' : 'Jira reads this as wiki markup.'}
        </span>
        <span id={ids.count} className="tabular-nums">
          {length === 1 ? '1 character' : `${String(length)} characters`}
        </span>
      </p>
      <Button variant="primary" aria-disabled={busy} onClick={onSend}>
        {busy ? 'Commenting…' : 'Comment'}
      </Button>
    </div>
  )
}
