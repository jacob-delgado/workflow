import { render, screen } from '@testing-library/react'
import { CommentBody } from './WikiText.tsx'

// Past the sizes the markup patterns are kept within, a comment is drawn as
// the text it is: each guard is shown by markup left undrawn, not by a time.

test('a body too long to parse safely is drawn as plain text, though its lines are short', () => {
  // Arrange
  const body = 'Ship **it** today.\n'.repeat(4_000)

  // Act
  render(<CommentBody body={body} markdown={true} />)

  // Assert
  expect(screen.queryByRole('strong')).toBeNull()
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

test('a comment the thread has no parsing left for is drawn as plain text', () => {
  // Act
  render(<CommentBody body="Ship **it**" markdown={true} plain={true} />)

  // Assert
  expect(screen.queryByRole('strong')).toBeNull()
})
