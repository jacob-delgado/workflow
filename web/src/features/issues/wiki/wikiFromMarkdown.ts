// wikiFromMarkdown rewrites the Markdown a comment is written in as the wiki
// markup Jira renders, as jira.WikiFromMarkdown does before posting when
// jira.markdown_comments is on, so the composer's Preview shows what Jira
// will be sent.
//
// Trade-off TRADE-28: these rules are written again in internal/jira/wiki.go,
// and twin-named tests pin the two.
export function wikiFromMarkdown(markdown: string): string {
  const out: string[] = []
  let inFence = false

  for (const line of markdown.split('\n')) {
    const fence = fenceLine(line)
    if (fence !== null) {
      out.push(fence)
      inFence = !inFence
    } else {
      out.push(inFence ? line : convertLine(line))
    }
  }

  return out.join('\n')
}

// boldSentinel stands in for wiki-bold's single asterisk while the emphasis
// rewrites run, so the italic pass does not read a fresh bold as an italic.
const boldSentinel = '\u0000'

// maxFenceIndent is how far a fence may be indented before Markdown reads it
// as indented content instead.
const maxFenceIndent = 3

// Go's . matches anything but a newline, a carriage return among it, where
// JavaScript's stops at both, so every . here is written [^\n].

// Neither whitespace nor an asterisk, with Go's \s, which is ASCII
// whitespace alone, where JavaScript's is Unicode's.
const notSpaceOrStar = '[^\\t\\n\\f\\r *]'

const heading = /^(#{1,6}) ([^\n]*)$/
const unordered = /^[-*+] ([^\n]*)$/
const ordered = /^\d+\. ([^\n]*)$/
const strike = /~~([^\n]+?)~~/g
const boldItalic = /\*\*\*([^\n]+?)\*\*\*/g
const boldStar = /\*\*([^\n]+?)\*\*/g
const boldUnder = /(^|\W)__([^\n]+?)__(\W|$)/g
const italicStar = new RegExp(`\\*(${notSpaceOrStar}(?:[^*]*?${notSpaceOrStar})?)\\*`, 'g')
const inlineCode = /`([^`]+)`/g
const link = /(!?)\[([^\]]*)\]\(([^()]*(?:\([^()]*\)[^()]*)*)\)/g

// fenceLine turns a ``` fence into its wiki bracket, or is null for any other
// line.
function fenceLine(line: string): string | null {
  const indent = line.length - line.replace(/^ +/, '').length
  const trimmed = line.trim()
  if (indent > maxFenceIndent || !trimmed.startsWith('```')) {
    return null
  }

  const language = trimmed.slice(3).trim()

  return language === '' ? '{code}' : `{code:${language}}`
}

// convertLine rewrites one line outside a code block: its block marker, then
// the emphasis, code and links in its text.
function convertLine(line: string): string {
  const [prefix, content] = blockPrefix(line)

  return prefix + convertInline(content)
}

// blockPrefix reads a line's heading, blockquote or list marker.
function blockPrefix(line: string): [string, string] {
  const [, level, title] = heading.exec(line) ?? []
  if (level !== undefined && title !== undefined) {
    return [`h${String(level.length)}. `, title]
  }

  if (line.startsWith('> ')) {
    return ['bq. ', line.slice(2)]
  }

  const [, item] = unordered.exec(line) ?? []
  if (item !== undefined) {
    return ['* ', item]
  }

  const [, step] = ordered.exec(line) ?? []
  if (step !== undefined) {
    return ['# ', step]
  }

  return ['', line]
}

// convertInline rewrites a run of text, keeping each code span's content out
// of the emphasis rewrites.
function convertInline(text: string): string {
  return eachMatch(text, inlineCode, convertEmphasis, (code) => `{{${code[1] ?? ''}}}`)
}

// convertEmphasis rewrites links, images and emphasis in text with no code
// span. A link's URL and an image's source are copied as they are.
function convertEmphasis(text: string): string {
  return eachMatch(text, link, emphasize, ([, bang, label = '', url = '']) =>
    bang === '!' ? `!${url}!` : `[${emphasize(label)}|${url}]`,
  )
}

// eachMatch writes each match of pattern in text through matched, and the
// text between matches through between.
function eachMatch(
  text: string,
  pattern: RegExp,
  between: (text: string) => string,
  matched: (match: RegExpExecArray) => string,
): string {
  let out = ''
  let last = 0

  for (const match of text.matchAll(pattern)) {
    out += between(text.slice(last, match.index)) + matched(match)
    last = match.index + match[0].length
  }

  return out + between(text.slice(last))
}

// emphasize rewrites strikethrough, bold and italic, parking bold on a
// sentinel first so the italic pass leaves it be.
function emphasize(text: string): string {
  return text
    .replace(strike, '-$1-')
    .replace(boldItalic, `${boldSentinel}_$1_${boldSentinel}`)
    .replace(boldStar, `${boldSentinel}$1${boldSentinel}`)
    .replace(boldUnder, `$1${boldSentinel}$2${boldSentinel}$3`)
    .replace(italicStar, '_$1_')
    .replaceAll(boldSentinel, '*')
}
