import { cn, plural } from './utils.ts'

test.each([
  ['gap-item', 'gap-section', 'gap-section'],
  ['gap-2', 'gap-group', 'gap-group'],
  ['mt-block', 'mt-4', 'mt-4'],
  ['-mb-block', '-mb-tight', '-mb-tight'],
])('a later spacing step wins over an earlier one: %s then %s', (earlier, later, merged) => {
  // Act & Assert
  expect(cn(earlier, later)).toBe(merged)
})

test.each([
  [1, 'file', '1 file'],
  [3, 'file', '3 files'],
  [0, 'hook', '0 hooks'],
])('counts %i %s in words: %s', (count, noun, said) => {
  // Act & Assert
  expect(plural(count, noun)).toBe(said)
})
