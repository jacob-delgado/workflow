import { stage, unstage } from '@/api/generated'

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
