import { render, screen, within } from '@testing-library/react'
import { WikiText } from './WikiText.tsx'

// renderWiki draws one comment body.
function renderWiki(markup: string) {
  render(<WikiText markup={markup} />)
}

// paragraphs are the paragraphs drawn, in order.
function paragraphs(): HTMLElement[] {
  return screen.queryAllByRole('paragraph')
}

test('bold, italic, struck and code text are drawn as such', () => {
  // Act
  renderWiki('a *bold* and _soft_ word, -gone-, and {{x := 1}}')

  // Assert
  expect(screen.getByRole('strong').textContent).toBe('bold')
  expect(screen.getByRole('emphasis').textContent).toBe('soft')
  expect(screen.getByRole('deletion').textContent).toBe('gone')
  expect(screen.getByRole('code').textContent).toBe('x := 1')
})

test('markers inside a word are left as they are', () => {
  // Act
  renderWiki('a well-known-name and snake_case_word, 2 * 3 * 4')

  // Assert
  expect(paragraphs().map((line) => line.textContent)).toEqual([
    'a well-known-name and snake_case_word, 2 * 3 * 4',
  ])
  expect(screen.queryByRole('deletion')).toBeNull()
  expect(screen.queryByRole('emphasis')).toBeNull()
  expect(screen.queryByRole('strong')).toBeNull()
})

test('a labeled link and a bare one open in a new tab, without an opener', () => {
  // Act
  renderWiki('See [the docs|https://example.com/docs] or [mailto:ana@example.com]')

  // Assert
  const docs = screen.getByRole('link', { name: 'the docs' })
  expect(docs.getAttribute('href')).toBe('https://example.com/docs')
  expect(docs.getAttribute('target')).toBe('_blank')
  expect(docs.getAttribute('rel')).toBe('noopener noreferrer')
  expect(screen.getByRole('link', { name: 'mailto:ana@example.com' })).toBeTruthy()
})

test('a link to anything but the web or mail stays text', () => {
  // Act
  renderWiki('[click|javascript:alert(1)] [data:text/html,x]')

  // Assert
  expect(screen.queryByRole('link')).toBeNull()
  expect(paragraphs().map((line) => line.textContent)).toEqual([
    '[click|javascript:alert(1)] [data:text/html,x]',
  ])
})

test('an image is offered as a link to it, never loaded', () => {
  // Act
  renderWiki('!https://example.com/shot.png!')

  // Assert
  expect(screen.queryByRole('img')).toBeNull()
  expect(screen.getByRole('link', { name: 'https://example.com/shot.png' })).toBeTruthy()
})

test('headings, a quote and lists are drawn as blocks', () => {
  // Act
  renderWiki('h2. Plan\nbq. said before\n* one\n* two\n# first\n# second')

  // Assert
  expect(paragraphs().map((line) => line.textContent)).toEqual(['Plan'])
  expect(screen.getByRole('blockquote').textContent).toBe('said before')
  const lists = screen.getAllByRole('list')
  expect(lists.map((list) => list.tagName)).toEqual(['UL', 'OL'])
  expect(
    lists.map((list) =>
      within(list)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ),
  ).toEqual([
    ['one', 'two'],
    ['first', 'second'],
  ])
})

test('a code block keeps its text as it is', () => {
  // Act
  renderWiki('before\n{code:go}\nx := *notBold*\n{code}\nafter')

  // Assert
  expect(screen.getByRole('code').textContent).toBe('x := *notBold*')
  expect(screen.queryByRole('strong')).toBeNull()
  expect(paragraphs().map((line) => line.textContent)).toEqual(['before', 'after'])
})

test('a code block never closed runs to the end', () => {
  // Act
  renderWiki('{code}\nstill code')

  // Assert
  expect(screen.getByRole('code').textContent).toBe('still code')
})

test('lines of a paragraph keep their breaks, and a blank line starts another', () => {
  // Act
  renderWiki('one\ntwo\n\nthree')

  // Assert
  const [first, second] = paragraphs()
  expect(first?.textContent).toBe('onetwo')
  expect(Array.from(first?.children ?? [], (child) => child.tagName)).toEqual(['BR'])
  expect(second?.textContent).toBe('three')
})

test('markup that never closes, and markup outside these rules, stay literal', () => {
  // Act
  renderWiki('an *open bold and {color:red}red{color} and <b>tag</b>')

  // Assert
  expect(paragraphs().map((line) => line.textContent)).toEqual([
    'an *open bold and {color:red}red{color} and <b>tag</b>',
  ])
  expect(screen.queryByRole('strong')).toBeNull()
})

test('emphasis inside a link label is drawn too', () => {
  // Act
  renderWiki('[*bold* docs|https://example.com]')

  // Assert
  const link = screen.getByRole('link', { name: 'bold docs' })
  expect(within(link).getByRole('strong').textContent).toBe('bold')
})

test('a comment full of markup that never closes is drawn at once, as text', () => {
  // Arrange
  // A hostile or pasted comment must not hang the page: every opener here is
  // left open, which a pattern that rescans from each one would pay for
  // again and again.
  const unclosed = '{{'.repeat(50_000)
  const started = performance.now()

  // Act
  renderWiki(unclosed)

  // Assert
  expect(performance.now() - started).toBeLessThan(500)
  expect(paragraphs().map((line) => line.textContent)).toEqual([unclosed])
})

test('a comment Jira sends with Windows line endings is drawn the same', () => {
  // Act
  renderWiki('h2. Plan\r\n* one\r\n* two\r\n{code}\r\nx := 1\r\n{code}')

  // Assert
  expect(paragraphs().map((line) => line.textContent)).toEqual(['Plan'])
  expect(
    within(screen.getByRole('list'))
      .getAllByRole('listitem')
      .map((item) => item.textContent),
  ).toEqual(['one', 'two'])
  expect(screen.getByRole('code').textContent).toBe('x := 1')
})
