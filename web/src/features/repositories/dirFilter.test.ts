import { z } from 'zod'
import type { DirectoryEntry } from '@/api/generated/types.gen.ts'
import { twinCases } from '@/test/twinCases.ts'
import { aboveTyped, matching, splitTyped, trailOf } from './dirFilter.ts'

// The cases internal/workdirs answers to as well.
const corpus = twinCases(
  'testdata/twins/directory_match.json',
  z.strictObject({
    about: z.string(),
    cases: z.array(
      z.strictObject({
        name: z.string(),
        listed: z.array(z.string()),
        typed: z.string(),
        want: z.array(
          z.strictObject({ name: z.string(), letters: z.array(z.number()), prefix: z.boolean() }),
        ),
      }),
    ),
  }),
)

// entry is a directory in /src by its name.
function entry(name: string): DirectoryEntry {
  return { name, path: `/src/${name}`, repository: false }
}

test.each(corpus.cases)('$name', ({ listed, typed, want }) => {
  // Arrange
  const entries = listed.map(entry)

  // Act
  const matches = matching(entries, typed)

  // Assert
  expect(
    matches.map(({ entry: { name }, letters, prefix }) => ({ name, letters, prefix })),
  ).toEqual(want)
})

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
