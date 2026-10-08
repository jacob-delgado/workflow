import { expect, test, type ConsoleMessage } from '@playwright/test'
import { openSection, pinTheme } from '../cockpit.ts'

// policyViolations collects what the browser reports refusing under the
// server's content policy, from the moment the page starts to load.
function policyViolations(messages: ConsoleMessage[]): string[] {
  return messages
    .map((message) => message.text())
    .filter((text) => text.includes('Content Security Policy'))
}

// The served page runs under the server's content policy, which runs no
// script written into the page or built from a string: the theme is resolved
// before paint by a script of its own, and the parsers compile nothing.
test('the page the server serves loads themed, with nothing its content policy refuses', async ({
  page,
}) => {
  // Arrange: a dark choice saved, and the console heard from the start.
  const messages: ConsoleMessage[] = []
  page.on('console', (message) => messages.push(message))
  await pinTheme(page, 'dark')

  // Act: load the page, and open a section the stream's frames fill.
  await page.goto('/')
  await openSection(page, 'Branch')

  // Assert: themed before paint, and nothing refused.
  // eslint-disable-next-line no-restricted-syntax -- data-theme is the resolved theme itself, the value the pre-paint script sets; no role, name or text carries it
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  expect(policyViolations(messages)).toEqual([])
})
