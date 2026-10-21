import { vi } from 'vitest'
import type { Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'

// A route's answer: a JSON body, or a function of the request's URL (and the
// request) for a route whose answer depends on its query or its method (a page
// of issues, a read that works beside a write that is refused). A function may
// return a Response of its own, for a route that refuses (a 409 problem, say),
// and may return a promise of either, for an answer a test holds back.
type Answer = unknown

// FakeApiOptions is how the fake server pushes the frame a write's answer
// wakes: 'pushed', as the server does, the snapshot already on the page again
// once the answer has gone; or 'held', for a test that pushes the frame that
// agrees with the write itself, so it can see what the page says before it.
export interface FakeApiOptions {
  frames: 'pushed' | 'held'
}

// fakeApi stands in for the server behind fetch, which test-setup otherwise
// refuses. Each request is answered with the route matching its path as a JSON
// body — a function route is called with the request's URL — and any path with
// no route answers 404. Every request is recorded, in order, so a test can say
// what the page asked the server for. As the server does, every answer to a
// write wakes the event stream: unless frames are held, the snapshot on the
// page is pushed again, a frame of its own, once the answer has gone.
export function fakeApi(
  routes: Record<string, Answer>,
  options: FakeApiOptions = { frames: 'pushed' },
): Request[] {
  const requests: Request[] = []

  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      requests.push(request)
      const answer = answerOf(routes, request)
      if (options.frames === 'pushed' && !readMethods.has(request.method)) {
        answer.then(pushTheFrameWoken, () => undefined)
      }

      return answer
    }),
  )

  return requests
}

// The methods that change nothing, as the server counts them.
const readMethods = new Set(['GET', 'HEAD', 'OPTIONS'])

// answerOf is the route's answer to request, or a 404 for a path with none.
function answerOf(routes: Record<string, Answer>, request: Request): Promise<Response> {
  const url = new URL(request.url)
  if (!(url.pathname in routes)) {
    return Promise.resolve(Response.json({ detail: 'no such route' }, { status: 404 }))
  }

  const route = routes[url.pathname]
  const body: unknown =
    typeof route === 'function'
      ? (route as (at: URL, asked: Request) => unknown)(url, request)
      : route

  return Promise.resolve(body).then((answered) =>
    answered instanceof Response ? answered : Response.json(answered),
  )
}

// pushTheFrameWoken pushes the snapshot on the page again, as a frame of its
// own, on a task after the one that answered: the page has read the answer's
// headers by then, as a browser reads them before the stream's next message.
function pushTheFrameWoken(): void {
  setTimeout(() => {
    pushFrame()
  }, 0)
}

// pushFrame pushes snapshot as the stream's next frame, as the stream hook
// keeps one: the snapshot on the page again, as a new frame, when none is
// given. With no snapshot on the page there is no stream to push to.
export function pushFrame(snapshot?: Snapshot): void {
  const shown = snapshot ?? useSnapshotStore.getState().snapshot
  if (shown === null) {
    return
  }

  useSnapshotStore.setState({ snapshot: { ...shown }, status: 'live', receivedAt: Date.now() })
}

// Held is an answer a test holds back: the promise a write waits on, and how
// to settle it once the test has seen the write's control while it runs.
export interface Held<T> {
  promise: Promise<T>
  answer: (value: T) => void
  refuse: (reason: unknown) => void
}

// held is an answer the test gives — or refuses — when it chooses, so a write
// is seen running before it is seen refused.
export function held<T>(): Held<T> {
  let answer: (value: T) => void = () => {}
  let refuse: (reason: unknown) => void = () => {}
  const promise = new Promise<T>((resolve, reject) => {
    answer = resolve
    refuse = reject
  })

  return { promise, answer, refuse }
}
