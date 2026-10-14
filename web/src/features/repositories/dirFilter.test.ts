import type { DirectoryEntry } from '@/api/generated/types.gen.ts'
import { aboveTyped, matching, splitTyped, trailOf } from './dirFilter.ts'

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
  ['~/src/api', '/home/ana/src/api', ['~/', '~/src/'], 'api'],
  ['/etc/nginx', '/etc/nginx', ['/', '/etc/'], 'nginx'],
  ['/', '/', [], '/'],
  // Your home is written from the root, so the way up goes on past it.
  ['~', '/home/ana', ['/', '/home/'], 'ana'],
])('%j, at %j, has above it %j and is named %j', (shown, path, typed, name) => {
  // Act
  const trail = trailOf({ path, shown, parent: '', entries: [], truncated: false })

  // Assert
  expect(trail.above.map((above) => above.typed)).toEqual(typed)
  expect(trail.name).toBe(name)
})

test.each([
  ['~/src/api/', '~/src/'],
  ['~/src/', '~/'],
  ['/etc/', '/'],
  ['src/', ''],
  ['../', '../../'],
  ['~/src/./', '~/src/./../'],
  ['/', undefined],
  // Where your home is, only its listing says.
  ['~/', undefined],
])('%j, before it is listed, has above it %j', (typed, above) => {
  // Act & Assert
  expect(aboveTyped(typed)).toBe(above)
})

test('a letter is matched where it stands in the name, whatever lowercasing makes of it', () => {
  // Arrange
  // İ lowercases to two letters, i and a combining dot, which must not push z
  // one place on.
  const entries = [entry('İzmir')]

  // Act
  const matches = matching(entries, 'Z')

  // Assert
  expect(matches.map((match) => match.letters)).toEqual([[1]])
})
