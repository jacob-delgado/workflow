import type { ReactNode } from 'react'

// WikiText draws a comment body written in Jira's wiki markup: bold, italic,
// struck and code text, links, headings, quotes, lists and code blocks. It
// builds elements alone, never HTML from the text, so nothing in a comment
// can reach the page as markup; anything outside these rules — a table, a
// panel, a color — is shown as the text it is.
export function WikiText({ markup }: { markup: string }) {
  return (
    <div className="flex min-w-0 flex-col gap-item break-words">
      {blocksOf(markup).map((block, index) => (
        // Blocks are drawn from the text alone, so their order is their identity.
        <BlockView key={index} block={block} />
      ))}
    </div>
  )
}

type Block =
  | { kind: 'paragraph'; lines: string[] }
  | { kind: 'heading'; text: string }
  | { kind: 'quote'; text: string }
  | { kind: 'list'; ordered: boolean; items: string[] }
  | { kind: 'code'; lines: string[] }

const codeFence = /^\{code(?::[^}]*)?\}$/
const headingLine = /^h[1-6]\. (.*)$/
const quoteLine = /^bq\. (.*)$/
const listLine = /^([*#])[*#]* (.*)$/

// blocksOf cuts markup into its blocks, line by line. A code block runs to its
// closing fence, or to the end when it has none.
function blocksOf(markup: string): Block[] {
  const blocks: Block[] = []
  let code: string[] | null = null

  for (const line of markup.split('\n')) {
    if (codeFence.test(line.trim())) {
      code = code === null ? [] : (blocks.push({ kind: 'code', lines: code }), null)
    } else if (code !== null) {
      code.push(line)
    } else {
      addLine(blocks, line)
    }
  }

  if (code !== null) {
    blocks.push({ kind: 'code', lines: code })
  }

  return blocks
}

// addLine adds one line outside a code block: a heading, a quote or a list
// item stands alone, a blank line ends a paragraph, and any other line joins
// the paragraph it follows.
function addLine(blocks: Block[], line: string) {
  const own = ownBlock(line)
  const last = blocks.at(-1)

  if (own?.kind === 'list') {
    addItem(blocks, own.ordered, own.items[0] ?? '')
  } else if (own !== null) {
    blocks.push(own)
  } else if (line.trim() === '') {
    blocks.push({ kind: 'paragraph', lines: [] })
  } else if (last?.kind === 'paragraph') {
    last.lines.push(line)
  } else {
    blocks.push({ kind: 'paragraph', lines: [line] })
  }
}

// ownBlock is the block a heading, quote or list item line makes, or null for
// a line of a paragraph.
function ownBlock(line: string): Block | null {
  const [, heading] = headingLine.exec(line) ?? []
  const [, quote] = quoteLine.exec(line) ?? []
  const [, marker, item] = listLine.exec(line) ?? []

  if (heading !== undefined) {
    return { kind: 'heading', text: heading }
  }

  if (quote !== undefined) {
    return { kind: 'quote', text: quote }
  }

  return marker === undefined || item === undefined
    ? null
    : { kind: 'list', ordered: marker === '#', items: [item] }
}

// addItem adds a list item to the list it follows, or starts one. Nested
// items are drawn at one level.
function addItem(blocks: Block[], ordered: boolean, item: string) {
  const last = blocks.at(-1)
  if (last?.kind === 'list' && last.ordered === ordered) {
    last.items.push(item)
  } else {
    blocks.push({ kind: 'list', ordered, items: [item] })
  }
}

function BlockView({ block }: { block: Block }) {
  switch (block.kind) {
    case 'paragraph':
      return block.lines.length === 0 ? null : <p>{withBreaks(block.lines)}</p>
    case 'heading':
      return <p className="font-semibold text-foreground">{inline(block.text)}</p>
    case 'quote':
      return (
        <blockquote className="border-l-2 border-border pl-3 text-muted-foreground">
          {inline(block.text)}
        </blockquote>
      )
    case 'list':
      return <ListView ordered={block.ordered} items={block.items} />
    case 'code':
      return (
        <pre className="overflow-x-auto rounded-md bg-muted px-3 py-2 text-xs">
          <code>{block.lines.join('\n')}</code>
        </pre>
      )
  }
}

function ListView({ ordered, items }: { ordered: boolean; items: string[] }) {
  const List = ordered ? 'ol' : 'ul'

  return (
    <List className={ordered ? 'list-decimal pl-5' : 'list-disc pl-5'}>
      {items.map((item, index) => (
        <li key={index}>{inline(item)}</li>
      ))}
    </List>
  )
}

// withBreaks draws a paragraph's lines with the breaks they were written with.
function withBreaks(lines: string[]): ReactNode[] {
  return lines.flatMap((line, index) =>
    index === 0 ? [inline(line)] : [<br key={index} />, inline(line)],
  )
}

// inlineMarkup matches one piece of inline markup. Emphasis markers count only
// at a word's edge, so snake_case, a hyphenated-word and 2 * 3 stay text.
const inlineMarkup = new RegExp(
  [
    String.raw`\{\{(?<code>.+?)\}\}`,
    String.raw`\[(?<label>[^\]|]+)\|(?<href>[^\]\s]+)\]`,
    String.raw`\[(?<bare>[^\]\s|]+)\]`,
    String.raw`!(?<image>[^!\s]+)!`,
    String.raw`(?<![\w*])\*(?<bold>[^*\s](?:[^*]*?[^*\s])?)\*(?![\w*])`,
    String.raw`(?<![\w_])_(?<italic>[^_\s](?:[^_]*?[^_\s])?)_(?![\w_])`,
    String.raw`(?<![\w-])-(?<strike>[^-\s](?:[^-]*?[^-\s])?)-(?![\w-])`,
  ].join('|'),
  'g',
)

// inline draws a run of text with its inline markup.
function inline(text: string): ReactNode {
  const parts: ReactNode[] = []
  let last = 0

  for (const match of text.matchAll(inlineMarkup)) {
    parts.push(text.slice(last, match.index), <Piece key={match.index} match={match} />)
    last = match.index + match[0].length
  }

  parts.push(text.slice(last))

  return parts
}

// Piece draws one match of inlineMarkup.
function Piece({ match }: { match: RegExpExecArray }) {
  const { code, bold, italic, strike } = match.groups ?? {}

  if (code !== undefined) {
    return <code className="rounded-sm bg-muted px-1 text-xs">{code}</code>
  }

  if (bold !== undefined) {
    return <strong>{inline(bold)}</strong>
  }

  if (italic !== undefined) {
    return <em>{inline(italic)}</em>
  }

  return strike === undefined ? <LinkPiece match={match} /> : <del>{inline(strike)}</del>
}

// LinkPiece draws a link or an image as a link, or its own text when it leads
// somewhere a comment may not send a reader. An image is never loaded.
function LinkPiece({ match }: { match: RegExpExecArray }) {
  const { label, href, bare, image } = match.groups ?? {}
  const target = href ?? bare ?? image ?? ''

  return isSafeLink(target) ? (
    <WikiLink href={target}>{inline(label ?? target)}</WikiLink>
  ) : (
    match[0]
  )
}

// isSafeLink admits a link to the web or to mail, and nothing a browser would
// run or render in place.
function isSafeLink(href: string): boolean {
  return /^(?:https?:\/\/|mailto:)/i.test(href)
}

function WikiLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="text-primary underline underline-offset-2 hover:no-underline"
    >
      {children}
    </a>
  )
}
