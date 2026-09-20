// apiErrorMessage pulls the human-readable reason from a thrown API error and
// falls back when there is none. The typed client throws the error body as-is —
// the API's { code, message } shape — rather than as an Error, so both are
// handled. Shared by the write actions that surface a refusal to the user.
export function apiErrorMessage(caught: unknown, fallback: string): string {
  if (caught instanceof Error) {
    return caught.message
  }

  if (typeof caught === 'object' && caught !== null && 'message' in caught) {
    if (typeof caught.message === 'string') {
      return caught.message
    }
  }

  return fallback
}
