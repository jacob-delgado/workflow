import { cn, keyedByText, plural } from './utils.ts'

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

test('repeated text is keyed apart by how often it came before', () => {
  // Act
  const keyed = keyedByText([' a', '+b', ' a'], (line) => line)

  // Assert
  expect(new Set(keyed.map(({ key }) => key)).size).toBe(3)
  expect(keyed.map(({ item }) => item)).toEqual([' a', '+b', ' a'])
})

test('an item keeps its key when another before it goes', () => {
  // Arrange
  const before = keyedByText(['one', 'two', 'three'], (word) => word)

  // Act
  const after = keyedByText(['one', 'three'], (word) => word)

  // Assert
  expect(after[1]?.key).toBe(before[2]?.key)
})
