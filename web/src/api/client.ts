import { HeldBack } from './apiError.ts'
import { client } from './generated/client.gen.ts'
import { useHealthStore } from './health.ts'
import { adoptSession, useSessionStore } from './session.ts'
import { useSnapshotStore } from './snapshot.ts'

// The session the address the page was opened at carries, or the one kept
// from before, taken before the first request leaves.
useSessionStore.setState({ token: adoptSession() })

// The SPA is served same-origin — by `workflow --web` in production, and through
// the Vite dev proxy in development — so API requests are relative. The
// generated client bakes in the contract's absolute loopback server URL, which
// in dev would bypass the proxy and hit a CORS wall; make every request relative
// to the page's own origin instead. Every request presents the page's session,
// where the contract's security scheme asks for it; with none, it presents
// nothing, and the server refuses it.
client.setConfig({
  baseUrl: '',
  auth: () => {
    const { token } = useSessionStore.getState()

    return token === '' ? undefined : token
  },
})

// An answer refusing the session marks it refused, whichever request met it,
// so the shell can say how to open the page with one.
client.interceptors.response.use((response) => {
  if (response.status === 401) {
    useSessionStore.setState({ refused: true })
  }

  return response
})

// dryRunHold is what a write says when --dry-run holds it back. It becomes the
// thrown hold's message, so every write surfaces it through apiErrorMessage
// the way it surfaces a server's refusal.
const dryRunHold = 'Held back by --dry-run: nothing was sent.'

// The methods that change nothing, as the server's own dry-run guard counts
// them; every other request is a write.
const safeMethods = new Set(['GET', 'HEAD', 'OPTIONS'])

// Under --dry-run every write is held back here, before it leaves the browser:
// one gate over every write, present and future, as the server keeps one guard
// over every handler. The write's button still answers — with the hold, as the
// terminal narrates a dry-run write — rather than going disabled.
client.interceptors.request.use((request) => {
  if (useHealthStore.getState().health?.dry_run === true && !safeMethods.has(request.method)) {
    throw new HeldBack(dryRunHold)
  }

  return request
})

// Every write names the directory the page shows, so a write made before the
// page has noticed a switch is refused rather than made in the directory
// switched to. A header carries bytes, not text, so the path goes escaped and
// the server unescapes it.
client.interceptors.request.use((request) => {
  const here = useSnapshotStore.getState().snapshot?.here
  if (here !== undefined && here !== '' && !safeMethods.has(request.method)) {
    request.headers.set('Workflow-Here', encodeURIComponent(here))
  }

  return request
})
