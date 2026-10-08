import { discard, getChangeDiff, stage, unstage } from '@/api/generated'
import type { FileDiff } from '@/api/generated/types.gen.ts'

// stageFile takes a changed file into the index, named by the path the working
// tree lists it under; the server finds the change itself, so a rename stages
// both of its paths. A refusal — a path the tree does not list, a file git
// will not stage — throws the API error, whose message is safe to show. On
// success the event stream reflects the index.
export async function stageFile(path: string): Promise<void> {
  await stage({ body: { path }, throwOnError: true })
}

// unstageFile takes a file's staged changes out of the index, leaving the work
// tree as it is.
export async function unstageFile(path: string): Promise<void> {
  await unstage({ body: { path }, throwOnError: true })
}

// stageEverything stages every change the index does not hold yet — a
// conflict included, which staging marks resolved — as the terminal's `a`
// does.
export async function stageEverything(): Promise<void> {
  await stage({ body: { all: true }, throwOnError: true })
}

// unstageEverything takes every staged change out of the index, leaving the
// work tree as it is, as the terminal's `U` does.
export async function unstageEverything(): Promise<void> {
  await unstage({ body: { all: true }, throwOnError: true })
}

// discardFile drops a changed file's changes from the index and the work tree,
// which cannot be undone: a tracked file goes back to the last commit, an
// untracked one is deleted. The file is named and found as for stageFile.
export async function discardFile(path: string): Promise<void> {
  await discard({ body: { path }, throwOnError: true })
}

// readDiff reads a changed file's diff against HEAD, on demand, the file named
// by the path the working tree lists it under. A refusal throws the API
// error, whose message is safe to show.
export async function readDiff(path: string): Promise<FileDiff> {
  const result = await getChangeDiff({ query: { path }, throwOnError: true })

  return result.data
}
