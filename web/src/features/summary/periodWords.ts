import type { Period } from './civilDate.ts'

// periodWords is a period as words say it: its one day, or its first day to
// its last, each day as dayWords writes it — a heading's way, or running
// text's.
export function periodWords(period: Period, dayWords: (date: string) => string): string {
  return period.from === period.to
    ? dayWords(period.from)
    : `${dayWords(period.from)} to ${dayWords(period.to)}`
}
