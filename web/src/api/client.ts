import { client } from './generated/client.gen.ts'

// The SPA is served same-origin — by `workflow --web` in production, and through
// the Vite dev proxy in development — so API requests are relative. The
// generated client bakes in the contract's absolute loopback server URL, which
// in dev would bypass the proxy and hit a CORS wall; make every request relative
// to the page's own origin instead.
client.setConfig({ baseUrl: '' })
