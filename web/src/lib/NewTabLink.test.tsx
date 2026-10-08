import { render, screen } from '@testing-library/react'
import { NewTabLink } from './NewTabLink.tsx'

test.each(['https://example.com/pull/42', 'http://jira.example.com/browse/PROJ-1'])(
  'a web address, %s, is a link that opens in a new tab',
  (href) => {
    // Act
    render(<NewTabLink href={href}>The page</NewTabLink>)

    // Assert
    const link = screen.getByRole('link', { name: 'The page (opens in a new tab)' })
    expect(link.getAttribute('href')).toBe(href)
    expect(link.getAttribute('target')).toBe('_blank')
  },
)

test.each([
  'data:text/html,<p>hi</p>',
  'ftp://example.com/build.log',
  'javascript:alert(1)',
  'mailto:ana@example.com',
  '/api/health',
  '',
])('any other address, "%s", is drawn as the words alone', (href) => {
  // Act
  render(<NewTabLink href={href}>The page</NewTabLink>)

  // Assert
  expect(screen.queryByRole('link')).toBeNull()
  expect(screen.getByText('The page')).toBeTruthy()
})
