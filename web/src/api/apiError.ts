// Refusal is a refusal worded by the page's own code, as the server words
// one — the mockup's stand-in for the server — so its message is meant to be
// shown.
export class Refusal extends Error {}

// HeldBack is a write the page itself held back before it left the browser —
// under --dry-run — so its message is the page's own words, meant to be shown.
export class HeldBack extends Refusal {}

// apiErrorMessage pulls the human-readable reason from a thrown API error and
// falls back when there is none. The typed client throws the error body as-is —
// an RFC 9457 problem details object ({ type, title, status, detail, code }) —
// rather than as an Error, so the problem's detail is preferred, then its title.
// A hold, or another refusal the page words, says what it is; any other thrown Error — a fetch that never reached
// the server, an answer that fails the schema — carries no reason fit to show,
// so the fallback says what to do instead. Shared by the write actions that
// surface a refusal to the user.
export function apiErrorMessage(caught: unknown, fallback: string): string {
  if (caught instanceof Refusal) {
    return caught.message
  }

  if (caught instanceof Error) {
    return fallback
  }

  const reason = problemReason(caught)

  return reason === '' ? fallback : reason
}

// problemReason is a problem details object's detail, else its title — the
// empty string when it has neither, or is no problem at all.
function problemReason(caught: unknown): string {
  if (typeof caught !== 'object' || caught === null) {
    return ''
  }

  if ('detail' in caught && typeof caught.detail === 'string' && caught.detail !== '') {
    return caught.detail
  }

  return 'title' in caught && typeof caught.title === 'string' ? caught.title : ''
}
