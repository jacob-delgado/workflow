import { discard, getChangeDiff, stage, unstage } from '@/api/generated'
import type { FileDiff } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and drops the SDK call from a production build's mock path, while
// tests can still stub the module.

// stageFile takes a changed file into the index, named by the path the working
// tree lists it under; the server finds the change itself, so a rename stages
// both of its paths. A refusal — a path the tree does not list, a file git
// will not stage — throws the API error, whose message is safe to show. On
// success the event stream reflects the index. Under VITE_MOCK it is a no-op.
export async function stageFile(path: string): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await stage({ body: { path }, throwOnError: true })
}

// unstageFile takes a file's staged changes out of the index, leaving the work
// tree as it is. Under VITE_MOCK it is a no-op.
export async function unstageFile(path: string): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await unstage({ body: { path }, throwOnError: true })
}

// stageEverything stages every change the index does not hold yet — a
// conflict included, which staging marks resolved — as the terminal's `a`
// does. Under VITE_MOCK it is a no-op.
export async function stageEverything(): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await stage({ body: { all: true }, throwOnError: true })
}

// unstageEverything takes every staged change out of the index, leaving the
// work tree as it is, as the terminal's `U` does. Under VITE_MOCK it is a
// no-op.
export async function unstageEverything(): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await unstage({ body: { all: true }, throwOnError: true })
}

// discardFile drops a changed file's changes from the index and the work tree,
// which cannot be undone: a tracked file goes back to the last commit, an
// untracked one is deleted. The file is named and found as for stageFile.
// Under VITE_MOCK it is a no-op.
export async function discardFile(path: string): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await discard({ body: { path }, throwOnError: true })
}

// readDiff reads a changed file's diff against HEAD, on demand, the file named
// by the path the working tree lists it under. A refusal throws the API
// error, whose message is safe to show. Under VITE_MOCK it answers a short
// diff, so the mockup can show one.
export async function readDiff(path: string): Promise<FileDiff> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return {
      path,
      lines: [
        `--- a/${path}`,
        `+++ b/${path}`,
        '@@ -1,3 +1,3 @@',
        ' package redact',
        '-const mask = "***"',
        '+const mask = "[redacted]"',
      ],
    }
  }

  const result = await getChangeDiff({ query: { path }, throwOnError: true })

  return result.data
}
