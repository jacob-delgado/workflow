import { create } from 'zustand'

// The key the session is kept under in this origin's storage, where a tab
// opened later at the same address finds it. Another port is another origin,
// so a page served elsewhere on this machine never reads it.
export const sessionStorageKey = 'workflow-session'

// What the address `workflow --web` prints carries the session after, in its
// fragment, which a browser sends to no server.
const fragmentPrefix = '#session='

interface SessionState {
  // The session every request presents, or '' when the page holds none.
  token: string
  // Whether the server refused the session the page presented, or that it
  // presented none: every read and write is refused until the page is opened
  // at the address the running server printed.
  refused: boolean
}

// The session this page presents to the server, and whether the server
// refused it. client.ts adopts the session as the page loads and marks a
// refusal from any answer; the shell says what to do about one.
export const useSessionStore = create<SessionState>(() => ({ token: '', refused: false }))

// adoptSession takes the session the page's address carries, when it carries
// one: it keeps it in this origin's storage, and takes it out of the address
// bar, so it is not left on screen or in a link copied from there. It answers
// the session the page presents from now on: the one adopted, else the one
// kept from before, else '' for none.
export function adoptSession(): string {
  const { hash, pathname, search } = window.location
  if (!hash.startsWith(fragmentPrefix)) {
    return kept()
  }

  const carried = hash.slice(fragmentPrefix.length)
  keep(carried)
  window.history.replaceState(window.history.state, '', pathname + search)

  return carried
}

// kept is the session this origin's storage holds, or '' when it holds none
// or cannot be read.
function kept(): string {
  try {
    return localStorage.getItem(sessionStorageKey) ?? ''
  } catch {
    return ''
  }
}

// keep stores session for a tab opened later; storage that is blocked keeps
// it for this page alone.
function keep(session: string): void {
  try {
    localStorage.setItem(sessionStorageKey, session)
  } catch {
    // Blocked storage: this page still presents what it adopted.
  }
}
