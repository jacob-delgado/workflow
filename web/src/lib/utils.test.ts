import { cn } from './utils.ts'

test.each([
  ['gap-item', 'gap-section', 'gap-section'],
  ['gap-2', 'gap-group', 'gap-group'],
  ['mt-block', 'mt-4', 'mt-4'],
  ['-mb-block', '-mb-tight', '-mb-tight'],
])('a later spacing step wins over an earlier one: %s then %s', (earlier, later, merged) => {
  // Act & Assert
  expect(cn(earlier, later)).toBe(merged)
})
