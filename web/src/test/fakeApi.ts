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
// no route answers 404, as the contract's validator does. Every request is
// recorded, in order, so a test can say what the page asked the server for. As
// the server does, every answer to a write it takes is numbered, in its
// Workflow-Write header, and wakes the event stream: unless frames are held,
// the snapshot on the page is pushed again, a frame of its own counting the
// writes answered, once the answer has gone. An answer a route numbered
// itself, as a run numbers its start and its end (landed), and one marked as
// the contract's refusal (refusedByTheContract) are not numbered again.
export function fakeApi(
  routes: Record<string, Answer>,
  options: FakeApiOptions = { frames: 'pushed' },
): Request[] {
  const requests: Request[] = []
  writesAnswered = 0

  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      requests.push(request)
      const write = !readMethods.has(request.method)
      const answer = answerOf(routes, request).then((answered) =>
        write ? numbered(answered) : answered,
      )
      if (options.frames === 'pushed' && write) {
        answer.then(pushTheFrameWoken, () => undefined)
      }

      return answer
    }),
  )

  return requests
}

// The methods that change nothing, as the server counts them.
const readMethods = new Set(['GET', 'HEAD', 'OPTIONS'])

// writeHeader is the header the server numbers a write's answer in.
const writeHeader = 'Workflow-Write'

// writesAnswered is how many writes the fake server has answered since the
// test set it up: what its next frame counts, and one less than the number its
// next write's answer carries.
let writesAnswered = 0

// refusals are the answers a test marked as the contract's refusals, which
// the validator gives before the server takes a request, so no write lands.
const refusals = new WeakSet<Response>()

// refusedByTheContract marks answer as the contract validator's refusal,
// which the server never numbers, since it never took the write.
export function refusedByTheContract(answer: Response): Response {
  refusals.add(answer)

  return answer
}

// landed counts a write the fake server answered, and is its number: for a
// route that numbers its own answer, as a run does its start and its end.
export function landed(): number {
  writesAnswered += 1

  return writesAnswered
}

// numbered is answer with the number it lands as, as the server numbers
// every write it answers; one the route numbered, or that the contract
// refused, it leaves as it is.
function numbered(answer: Response): Response {
  if (!refusals.has(answer) && !answer.headers.has(writeHeader)) {
    answer.headers.set(writeHeader, String(landed()))
  }

  return answer
}

// answerOf is the route's answer to request, or the contract's 404 for a path
// with none.
function answerOf(routes: Record<string, Answer>, request: Request): Promise<Response> {
  const url = new URL(request.url)
  if (!(url.pathname in routes)) {
    return Promise.resolve(
      refusedByTheContract(Response.json({ detail: 'no such route' }, { status: 404 })),
    )
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
// given. The frame is read after every write the fake has answered, and
// counts them. With no snapshot on the page there is no stream to push to.
export function pushFrame(snapshot?: Snapshot): void {
  const shown = snapshot ?? useSnapshotStore.getState().snapshot
  if (shown === null) {
    return
  }

  useSnapshotStore.setState({
    snapshot: { ...shown, writes: writesAnswered },
    status: 'live',
    receivedAt: Date.now(),
  })
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
