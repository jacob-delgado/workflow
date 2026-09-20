import { checkout } from '@/api/generated'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and drops the SDK call from a production build's mock path, while
// tests can still stub the module.

// checkoutBranch switches the working tree to a branch. On success the event
// stream reflects the switch, so there is nothing to return; a dirty tree or a
// failed switch throws the API error, whose message is safe to show. Under
// VITE_MOCK it is a no-op, so the mockup's button is inert rather than erroring
// against a backend that is not there.
export async function checkoutBranch(branch: string): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await checkout({ body: { branch }, throwOnError: true })
}

// refusalMessage pulls the human-readable reason from a thrown checkout error —
// the API's { code, message } body, or an Error — and falls back when there is
// none. Kept here rather than shared with Settings' equivalent until a third
// caller earns the extraction.
export function refusalMessage(caught: unknown): string {
  if (caught instanceof Error) {
    return caught.message
  }
  if (typeof caught === 'object' && caught !== null && 'message' in caught) {
    if (typeof caught.message === 'string') {
      return caught.message
    }
  }

  return 'The branch could not be checked out.'
}
