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
