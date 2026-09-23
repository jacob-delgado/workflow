import { apiErrorMessage, HeldBack } from './apiError.ts'

test('apiErrorMessage surfaces a hold, a problem detail or title, else the fallback', () => {
  // Act & Assert
  expect(apiErrorMessage(new HeldBack('held'), 'fallback')).toBe('held')
  // A fetch that never reached the server, or an answer that fails the schema,
  // throws an Error whose words are no reason to show.
  expect(apiErrorMessage(new TypeError('Failed to fetch'), 'fallback')).toBe('fallback')
  expect(
    apiErrorMessage(
      {
        type: 't',
        title: 'Conflict',
        status: 409,
        detail: 'a branch already exists',
        code: 'conflict',
      },
      'fallback',
    ),
  ).toBe('a branch already exists')
  expect(
    apiErrorMessage({ type: 't', title: 'Not found', status: 404, code: 'not_found' }, 'fallback'),
  ).toBe('Not found')
  expect(apiErrorMessage(42, 'the fallback')).toBe('the fallback')
})
