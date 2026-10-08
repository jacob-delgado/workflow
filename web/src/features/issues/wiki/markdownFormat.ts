// A Format is what one of the composer's formatting buttons writes.
export type Format = 'bold' | 'italic' | 'code' | 'link' | 'list'

// Edit is the text after a format is applied, and the part of it to leave
// selected: the words the format was applied to, or what is left to fill in.
export interface Edit {
  text: string
  start: number
  end: number
}

// marks are the Markdown each inline format wraps its words in, and the words
// it writes when nothing is selected.
const marks: Record<Exclude<Format, 'link' | 'list'>, { mark: string; placeholder: string }> = {
  bold: { mark: '**', placeholder: 'bold text' },
  italic: { mark: '*', placeholder: 'italic text' },
  code: { mark: '`', placeholder: 'code' },
}

// applyFormat writes format over text's selection from start to end, as
// Markdown, and leaves the words it was applied to selected.
export function applyFormat(format: Format, text: string, start: number, end: number): Edit {
  switch (format) {
    case 'link':
      return link(text, start, end)
    case 'list':
      return list(text, start, end)
    case 'bold':
    case 'italic':
    case 'code':
      return wrap(text, start, end, marks[format])
  }
}

function wrap(
  text: string,
  start: number,
  end: number,
  { mark, placeholder }: { mark: string; placeholder: string },
): Edit {
  const words = text.slice(start, end) || placeholder
  const opened = start + mark.length

  return {
    text: text.slice(0, start) + mark + words + mark + text.slice(end),
    start: opened,
    end: opened + words.length,
  }
}

// link makes the selection a link's label and selects the address to fill in.
function link(text: string, start: number, end: number): Edit {
  const label = text.slice(start, end) || 'link text'
  const address = 'https://'
  const opened = start + label.length + 3

  return {
    text: `${text.slice(0, start)}[${label}](${address})${text.slice(end)}`,
    start: opened,
    end: opened + address.length,
  }
}

// list marks each line the selection touches as a bullet, and selects them.
function list(text: string, start: number, end: number): Edit {
  const from = text.lastIndexOf('\n', start - 1) + 1
  const lineEnd = text.indexOf('\n', end)
  const to = lineEnd === -1 ? text.length : lineEnd
  const bullets = text
    .slice(from, to)
    .split('\n')
    .map((line) => `- ${line}`)
    .join('\n')

  return {
    text: text.slice(0, from) + bullets + text.slice(to),
    start: from,
    end: from + bullets.length,
  }
}
