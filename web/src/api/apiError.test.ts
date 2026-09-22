import { apiErrorMessage } from './apiError.ts'

test('apiErrorMessage surfaces a thrown Error, a problem detail or title, else the fallback', () => {
  // Act & Assert
  expect(apiErrorMessage(new Error('boom'), 'fallback')).toBe('boom')
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
