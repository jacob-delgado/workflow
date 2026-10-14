import type { DirectoryEntry, DirectoryListing } from '@/api/generated/types.gen.ts'

// Typed is what the directory field holds, read as the directory to list and
// the start of a name in it to narrow the list by. parent is '' for where the
// server works: a bare name narrows that.
interface Typed {
  parent: string
  segment: string
}

// splitTyped reads the field: up to its last slash, the directory to list —
// from where the server works, or from your home after ~ — and after it, the
// name to narrow by. A lone ~ is your home.
export function splitTyped(typed: string): Typed {
  if (typed === '~') {
    return { parent: '~', segment: '' }
  }

  const slash = typed.lastIndexOf('/')
  if (slash < 0) {
    return { parent: '', segment: typed }
  }

  return { parent: slash === 0 ? '/' : typed.slice(0, slash), segment: typed.slice(slash + 1) }
}

// Above is a directory above the one listed: what the field types to list
// it, and its name in the path.
interface Above {
  typed: string
  name: string
}

// Trail is the path to the directory listed: the directories above it, from
// the top, and its own name.
export interface Trail {
  above: Above[]
  name: string
}

// trailOf is the path to the directory listed as the server shows it, or —
// for your home, whose ~ has nothing above it — its path from the root, so
// the way up goes on past it, as it does in the terminal.
export function trailOf(listing: DirectoryListing): Trail {
  const path = listing.shown === '~' ? listing.path : listing.shown
  if (path === '/') {
    return { above: [], name: '/' }
  }

  const parts = path.split('/')
  const above = parts.slice(0, -1).map((part, index) => ({
    typed: `${parts.slice(0, index + 1).join('/')}/`,
    name: part === '' ? '/' : part,
  }))

  return { above, name: parts.at(-1) ?? '' }
}

// aboveTyped is the directory above the one typed, which ends with its slash,
// as the field types it: the way up before a listing has said where the one
// typed is. Past a dot or two it goes a level further, and nothing is above
// the root, nor your home, whose place only its listing says.
export function aboveTyped(typed: string): string | undefined {
  const dir = typed.slice(0, -1)
  const name = dir.slice(dir.lastIndexOf('/') + 1)
  if (dir === '' || dir === '~') {
    return undefined
  }

  return name === '.' || name === '..' ? `${typed}../` : dir.slice(0, dir.lastIndexOf('/') + 1)
}

// joined is a listing asked again by a name, with the rest of the first
// listing's after its own, so a match the first drew stays once the second
// answers; either alone until both have.
export function joined(
  first: DirectoryListing | undefined,
  narrowed: DirectoryListing | undefined,
): DirectoryListing | undefined {
  if (first === undefined || narrowed === undefined) {
    return narrowed ?? first
  }

  const found = new Set(narrowed.entries.map((entry) => entry.path))

  return {
    ...narrowed,
    entries: [...narrowed.entries, ...first.entries.filter((entry) => !found.has(entry.path))],
  }
}

// Match is a directory whose name holds what was typed, its letters in
// order: where each matched, and whether the name starts with them.
export interface Match {
  entry: DirectoryEntry
  letters: number[]
  prefix: boolean
}

// matching is the entries whose names hold typed's letters in order, in any
// case: those starting with typed first, then the rest, each as listed — the
// order workdirs.Matching ranks names in for the terminal.
//
// Trade-off TRADE-52: this matcher is written again in internal/workdirs, and
// both copies answer to the one case file testdata/twins/directory_match.json,
// so a change to either alone fails its own tests.
export function matching(entries: DirectoryEntry[], typed: string): Match[] {
  const matches = entries.flatMap((entry) => {
    const letters = inOrder(entry.name, typed)

    return letters === undefined
      ? []
      : [{ entry, letters, prefix: letters.every((place, index) => place === index) }]
  })

  return [...matches.filter((match) => match.prefix), ...matches.filter((match) => !match.prefix)]
}

// inOrder is where in name each of typed's letters is, in any case, each the
// first after the last, or undefined when name runs out first. Each letter is
// lowercased on its own, to one letter, so every place after it stays where
// the name has it.
function inOrder(name: string, typed: string): number[] | undefined {
  const wanted = Array.from(typed, lowered)
  const letters: number[] = []
  for (const [place, letter] of Array.from(name).entries()) {
    if (letters.length < wanted.length && lowered(letter) === wanted[letters.length]) {
      letters.push(place)
    }
  }

  return letters.length === wanted.length ? letters : undefined
}

// lowered is a letter's own lowercase, as Go's unicode.ToLower gives it: the
// first of what JavaScript lowers it to, since that lowers İ to i and a
// combining dot, and the i alone is what typing i means to find.
function lowered(letter: string): number | undefined {
  return letter.toLowerCase().codePointAt(0)
}
