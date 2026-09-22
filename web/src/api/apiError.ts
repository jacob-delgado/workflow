// apiErrorMessage pulls the human-readable reason from a thrown API error and
// falls back when there is none. The typed client throws the error body as-is —
// an RFC 9457 problem details object ({ type, title, status, detail, code }) —
// rather than as an Error, so the problem's detail is preferred, then its title,
// and a thrown Error's message is still handled. Shared by the write actions
// that surface a refusal to the user.
export function apiErrorMessage(caught: unknown, fallback: string): string {
  if (caught instanceof Error) {
    return caught.message
  }

  if (typeof caught === 'object' && caught !== null) {
    if ('detail' in caught && typeof caught.detail === 'string' && caught.detail !== '') {
      return caught.detail
    }

    if ('title' in caught && typeof caught.title === 'string' && caught.title !== '') {
      return caught.title
    }
  }

  return fallback
}
