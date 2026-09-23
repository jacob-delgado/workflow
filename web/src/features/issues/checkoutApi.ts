import { checkout } from '@/api/generated'
import type { Branch } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and drops the SDK call from a production build's mock path, while
// tests can still stub the module.

// checkoutBranch switches the working tree to a branch and returns the branch
// now checked out, for the button to say where it went; the event stream
// reflects the switch too. A dirty tree or a failed switch throws the API
// error, whose message is safe to show. Under VITE_MOCK it answers with the
// mockup's branch under the name asked for, rather than erroring against a
// backend that is not there.
export async function checkoutBranch(branch: string): Promise<Branch> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return { ...mockSnapshot.branch, name: branch }
  }

  const result = await checkout({ body: { branch }, throwOnError: true })

  return result.data
}
