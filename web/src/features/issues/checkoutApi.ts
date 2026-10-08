import { checkout } from '@/api/generated'
import type { Branch } from '@/api/generated/types.gen.ts'

// checkoutBranch switches the working tree to a branch and returns the branch
// now checked out, for the button to say where it went; the event stream
// reflects the switch too. A dirty tree or a failed switch throws the API
// error, whose message is safe to show.
export async function checkoutBranch(branch: string): Promise<Branch> {
  const result = await checkout({ body: { branch }, throwOnError: true })

  return result.data
}
