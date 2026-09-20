import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

// Merge Tailwind class lists, letting a later class win over an earlier one that
// sets the same property (clsx joins, tailwind-merge dedupes the conflict). The
// shadcn convention every component uses to make its classes overridable.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
