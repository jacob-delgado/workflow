import { render, screen } from '@testing-library/react'
import { CommentBody } from './WikiText.tsx'

// hostile is a body of openers that never close: each pattern that rescans
// from every opener makes it quadratic, which a thread written by anyone who
// can comment on the repository must not be able to reach.
const hostile = '[a|'.repeat(25_000)

test.each([
  ['wiki markup', false],
  ['Markdown', true],
])('a %s comment too large to parse safely is drawn as plain text, fast', (_, markdown) => {
  // Arrange
  const started = performance.now()

  // Act
  render(<CommentBody body={hostile} markdown={markdown} />)

  // Assert
  expect(performance.now() - started).toBeLessThan(500)
  expect(screen.getByText(hostile)).toBeTruthy()
})

test('a line too long to parse safely is drawn as plain text, though the body is short', () => {
  // Arrange
  const line = `*${'x'.repeat(1_001)}*`

  // Act
  render(<CommentBody body={line} markdown={false} />)

  // Assert
  expect(screen.queryByRole('strong')).toBeNull()
  expect(screen.getByText(line)).toBeTruthy()
})

test('a comment of ordinary size is drawn with its markup', () => {
  // Act
  render(<CommentBody body="Ship **it**" markdown={true} />)

  // Assert
  expect(screen.getByRole('strong').textContent).toBe('it')
})
