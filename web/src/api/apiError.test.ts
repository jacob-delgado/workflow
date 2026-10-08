import { apiErrorMessage, HeldBack, problemCode } from './apiError.ts'

const conflict = {
  type: 't',
  title: 'Conflict',
  status: 409,
  detail: 'a branch already exists',
  code: 'conflict',
}
const notFound = { type: 't', title: 'Not found', status: 404, code: 'not_found' }

test.each([
  { thrown: 'a hold', caught: new HeldBack('held'), said: 'held' },
  // A fetch that never reached the server, or an answer that fails the
  // schema, throws an Error whose words are no reason to show.
  { thrown: 'an Error', caught: new TypeError('Failed to fetch'), said: 'the fallback' },
  { thrown: 'a problem with a detail', caught: conflict, said: 'a branch already exists' },
  { thrown: 'a problem with only a title', caught: notFound, said: 'Not found' },
  {
    thrown: 'a problem with an empty detail',
    caught: { ...notFound, detail: '' },
    said: 'Not found',
  },
  { thrown: 'an empty object', caught: {}, said: 'the fallback' },
  { thrown: 'no object at all', caught: 42, said: 'the fallback' },
])('apiErrorMessage says what $thrown says: $said', ({ caught, said }) => {
  // Act & Assert
  expect(apiErrorMessage(caught, 'the fallback')).toBe(said)
})

test.each([
  { thrown: 'a problem', caught: conflict, code: 'conflict' },
  { thrown: 'a problem with no code', caught: { type: 't', title: 'Gone', status: 410 }, code: '' },
  { thrown: 'a problem whose code is no word', caught: { ...conflict, code: 7 }, code: '' },
  { thrown: 'an Error', caught: new HeldBack('held'), code: '' },
  { thrown: 'null', caught: null, code: '' },
  { thrown: 'no object at all', caught: 'conflict', code: '' },
])('problemCode reads $thrown as "$code"', ({ caught, code }) => {
  // Act & Assert
  expect(problemCode(caught)).toBe(code)
})
