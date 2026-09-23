import { HeldBack } from './apiError.ts'
import { client } from './generated/client.gen.ts'
import { useHealthStore } from './health.ts'

// The SPA is served same-origin — by `workflow --web` in production, and through
// the Vite dev proxy in development — so API requests are relative. The
// generated client bakes in the contract's absolute loopback server URL, which
// in dev would bypass the proxy and hit a CORS wall; make every request relative
// to the page's own origin instead.
client.setConfig({ baseUrl: '' })

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
