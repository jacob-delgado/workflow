// HeldBack is a write the page itself held back before it left the browser —
// under --dry-run — so its message is the page's own words, meant to be shown.
export class HeldBack extends Error {}

// apiErrorMessage pulls the human-readable reason from a thrown API error and
// falls back when there is none. The typed client throws the error body as-is —
// an RFC 9457 problem details object ({ type, title, status, detail, code }) —
// rather than as an Error, so the problem's detail is preferred, then its title.
// A hold, or another refusal the page words, says what it is; any other thrown
// Error — a fetch that never reached the server, an answer that fails the
// schema — carries no reason fit to show, so the fallback says what to do
// instead. Shared by the write actions that surface a refusal to the user.
export function apiErrorMessage(caught: unknown, fallback: string): string {
  if (caught instanceof HeldBack) {
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
  const detail = problemField(caught, 'detail')
  if (typeof detail === 'string' && detail !== '') {
    return detail
  }

  const title = problemField(caught, 'title')

  return typeof title === 'string' ? title : ''
}

// problemCode is a problem details object's machine code — fetch_failed, say —
// for a control that offers its own way out of one refusal; the empty string
// for anything else thrown.
export function problemCode(caught: unknown): string {
  const code = problemField(caught, 'code')

  return typeof code === 'string' ? code : ''
}

// mayAnswerAgain reports a failure that asking once more may get past: a
// request that never reached the server, which fetch fails with a TypeError,
// or a problem the server answered with a status of 500 or above — it, or
// the service behind it, failed in passing. A refusal (4xx) stands until
// something changes, and anything else thrown says nothing a second ask
// would change.
export function mayAnswerAgain(caught: unknown): boolean {
  if (caught instanceof TypeError) {
    return true
  }

  const status = problemField(caught, 'status')

  return typeof status === 'number' && status >= 500
}

// problemField is the field of a problem details object named, as the typed
// client throws one; undefined for anything else thrown, an Error included.
function problemField(caught: unknown, name: string): unknown {
  if (typeof caught !== 'object' || caught === null || caught instanceof Error) {
    return undefined
  }

  return name in caught ? (caught as Record<string, unknown>)[name] : undefined
}
