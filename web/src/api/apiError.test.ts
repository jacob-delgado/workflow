import { apiErrorMessage } from './apiError.ts'

test('apiErrorMessage surfaces a thrown Error, an error body, else the fallback', () => {
  // Act & Assert
  expect(apiErrorMessage(new Error('boom'), 'fallback')).toBe('boom')
  expect(
    apiErrorMessage({ code: 'conflict', message: 'a branch already exists' }, 'fallback'),
  ).toBe('a branch already exists')
  expect(apiErrorMessage(42, 'the fallback')).toBe('the fallback')
})
