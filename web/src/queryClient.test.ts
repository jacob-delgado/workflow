import { queryClient } from './queryClient.ts'

test('does not refetch on its own — the event stream keeps the cache fresh', () => {
  // Act
  const queries = queryClient.getDefaultOptions().queries

  // Assert
  expect(queries?.refetchOnWindowFocus).toBe(false)
  expect(queries?.staleTime).toBe(Infinity)
})
