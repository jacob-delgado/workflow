import { vi } from 'vitest'

// A route's answer: a JSON body, or a function of the request's URL (and the
// request) for a route whose answer depends on its query or its method (a page
// of issues, a read that works beside a write that is refused). A function may
// return a Response of its own, for a route that refuses (a 409 problem, say),
// and may return a promise of either, for an answer a test holds back.
type Answer = unknown

// fakeApi stands in for the server behind fetch, which test-setup otherwise
// refuses. Each request is answered with the route matching its path as a JSON
// body — a function route is called with the request's URL — and any path with
// no route answers 404. Every request is recorded, in order, so a test can say
// what the page asked the server for.
export function fakeApi(routes: Record<string, Answer>): Request[] {
  const requests: Request[] = []

  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      requests.push(request)
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
    }),
  )

  return requests
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
