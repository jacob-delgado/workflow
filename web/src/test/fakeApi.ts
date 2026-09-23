import { vi } from 'vitest'

// A route's answer: a JSON body, or a function of the request's URL (and the
// request) for a route whose answer depends on its query or its method (a page
// of issues, a read that works beside a write that is refused). A function may
// return a Response of its own, for a route that refuses (a 409 problem, say).
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

      return Promise.resolve(body instanceof Response ? body : Response.json(body))
    }),
  )

  return requests
}
