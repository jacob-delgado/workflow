import { clsx, type ClassValue } from 'clsx'
import { extendTailwindMerge } from 'tailwind-merge'

// tailwind-merge knows the numeric spacing steps; the named ones index.css
// declares — tight, item, group, block, section — are taught to it here, so a
// later `gap-section` replaces an earlier `gap-item` rather than both standing.
const twMerge = extendTailwindMerge({
  extend: { theme: { spacing: ['tight', 'item', 'group', 'block', 'section'] } },
})

// Merge Tailwind class lists, letting a later class win over an earlier one that
// sets the same property (clsx joins, tailwind-merge dedupes the conflict). The
// shadcn convention every component uses to make its classes overridable.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}

// definitionList lays out a <dl> of terms beside their values: the terms in a
// column as wide as the longest of them, the values in what is left of the
// width, wrapping within it.
export const definitionList =
  'grid grid-cols-[max-content_minmax(0,1fr)] gap-x-group gap-y-tight text-sm'

// contentMeasure is the one width a section's content is set at, whatever the
// window: a form, a list, a detail beside a list. Wider, a line of the body
// face runs past what an eye can follow back; a list beside a detail grows with
// the window instead, and the detail keeps this measure.
export const contentMeasure = 'max-w-2xl'

// capitalized starts text with a capital, for a word the server sends lowercase
// — the forge's "merge request" — that opens a sentence or names a stage.
export function capitalized(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1)
}

// splitList reads a comma-separated field into its trimmed, non-empty entries,
// so a stray comma never sends a blank reviewer, label or commit type.
export function splitList(text: string): string[] {
  return text
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry) => entry !== '')
}

// plural is a count and its noun in words, the noun taking an s for any count
// but one: 1 file, 3 files, 0 hooks — as the terminal's plural counts.
export function plural(count: number, noun: string): string {
  return `${String(count)} ${count === 1 ? noun : `${noun}s`}`
}
