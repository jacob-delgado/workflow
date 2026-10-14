import type { DirectoryEntry } from '@/api/generated/types.gen.ts'

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

// upFrom is the field one directory up from one that ends in a slash: the
// directory above, with its slash, or nothing for a bare name. Your home and
// the root stay as they are, for the breadcrumbs to go above.
export function upFrom(typed: string): string {
  if (typed === '/' || typed === '~/') {
    return typed
  }

  const above = typed.slice(0, -1).lastIndexOf('/')

  return above < 0 ? '' : typed.slice(0, above + 1)
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
export function matching(entries: DirectoryEntry[], typed: string): Match[] {
  const matches = entries.flatMap((entry) => {
    const letters = inOrder(entry.name.toLowerCase(), typed.toLowerCase())

    return letters === undefined
      ? []
      : [{ entry, letters, prefix: letters.every((place, index) => place === index) }]
  })

  return [...matches.filter((match) => match.prefix), ...matches.filter((match) => !match.prefix)]
}

// inOrder is where in name each of typed's letters is, each the first after
// the last, or undefined when name runs out first.
function inOrder(name: string, typed: string): number[] | undefined {
  const wanted = Array.from(typed)
  const letters: number[] = []
  for (const [place, letter] of Array.from(name).entries()) {
    if (letters.length < wanted.length && letter === wanted[letters.length]) {
      letters.push(place)
    }
  }

  return letters.length === wanted.length ? letters : undefined
}
