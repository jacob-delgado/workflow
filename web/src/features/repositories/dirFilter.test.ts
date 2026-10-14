import type { DirectoryEntry } from '@/api/generated/types.gen.ts'
import { matching, splitTyped, upFrom } from './dirFilter.ts'

// entry is a directory in /src by its name.
function entry(name: string): DirectoryEntry {
  return { name, path: `/src/${name}`, repository: false }
}

test.each([
  ['', '', ''],
  ['wo', '', 'wo'],
  ['~', '~', ''],
  ['~/', '~', ''],
  ['~/src/wo', '~/src', 'wo'],
  ['~/src/', '~/src', ''],
  ['/', '/', ''],
  ['/home', '/', 'home'],
  ['../api/c', '../api', 'c'],
])('%j lists %j and narrows it by %j', (typed, parent, segment) => {
  // Act & Assert
  expect(splitTyped(typed)).toEqual({ parent, segment })
})

test('names starting with what was typed come first, then those holding its letters in order', () => {
  // Arrange
  const entries = ['api-docs', 'web', 'workday', 'snowboard', 'workflow'].map(entry)

  // Act
  const matches = matching(entries, 'WO')

  // Assert
  expect(matches.map((match) => [match.entry.name, match.letters])).toEqual([
    ['workday', [0, 1]],
    ['workflow', [0, 1]],
    ['snowboard', [3, 5]],
  ])
})

test('every name matches nothing typed, as listed', () => {
  // Arrange
  const entries = ['web', 'api'].map(entry)

  // Act
  const matches = matching(entries, '')

  // Assert
  expect(matches.map((match) => match.entry.name)).toEqual(['web', 'api'])
})

test.each([
  ['~/src/', '~/'],
  ['~/src/api/', '~/src/'],
  ['/home/', '/'],
  ['~/', '~/'],
  ['/', '/'],
  ['api/', ''],
])('going up from %j leaves %j', (typed, up) => {
  // Act & Assert
  expect(upFrom(typed)).toBe(up)
})
