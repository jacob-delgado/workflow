import { queryClient } from './queryClient.ts'

test('by default a query does not refetch on its own — the event stream keeps the cache fresh', () => {
  // Act
  const queries = queryClient.getDefaultOptions().queries

  // Assert
  expect(queries?.refetchOnWindowFocus).toBe(false)
  expect(queries?.staleTime).toBe(Infinity)
})

// problem is the body of a problem answer of status.
function problem(status: number) {
  return { type: 'about:blank', title: 'Problem', status, detail: 'why', code: 'why' }
}

test.each([
  { failed: 'a refusal (4xx)', caught: problem(404), failures: 0, retried: false },
  { failed: 'a server failure (5xx)', caught: problem(502), failures: 0, retried: true },
  {
    failed: 'a network error',
    caught: new TypeError('Failed to fetch'),
    failures: 0,
    retried: true,
  },
  { failed: 'a server failure asked again', caught: problem(503), failures: 1, retried: false },
  {
    failed: 'an answer the schema refuses',
    caught: new Error('invalid'),
    failures: 0,
    retried: false,
  },
])('a read failing with $failed after $failures retries is retried: $retried', (row) => {
  // Arrange
  const retry = queryClient.getDefaultOptions().queries?.retry

  // Act
  // The client throws a problem as its body, which the query's types call an
  // Error.
  const retried = typeof retry === 'function' && retry(row.failures, row.caught as Error)

  // Assert
  expect(retried).toBe(row.retried)
})
