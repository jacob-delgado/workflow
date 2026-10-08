import { expect, test, type ConsoleMessage } from '@playwright/test'
import { openSection, pinTheme } from '../support/cockpit.ts'
import { openServed, printedAddress } from './served.ts'

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
  // Arrange
  // The console is heard from the start, so a refusal while the page loads is
  // caught.
  const messages: ConsoleMessage[] = []
  page.on('console', (message) => messages.push(message))
  await pinTheme(page, 'dark')

  // Act
  // Branch is a section the stream's frames fill.
  await openServed(page)
  await openSection(page, 'Branch')

  // Assert
  // eslint-disable-next-line no-restricted-syntax, playwright/no-raw-locators -- data-theme is the resolved theme itself, the value the pre-paint script sets; no role, name or text carries it
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  expect(policyViolations(messages)).toEqual([])
})

// The address the server prints carries the page's session in its fragment,
// which the browser sends to no server: the page keeps it, takes it out of
// the address bar, and presents it on every request and on the stream.
test('the page takes its session out of the address, and the stream goes live with it', async ({
  page,
}) => {
  // Act
  await openServed(page)

  // Assert
  await expect(page.getByRole('status').filter({ hasText: 'Live' })).toBeVisible()
  await expect(page).toHaveURL(new URL('/', printedAddress()).href)
})

test('a page opened without the session says to open the address the server printed', async ({
  page,
}) => {
  // Act
  // This browser holds no session.
  await page.goto('/')

  // Assert
  await expect(page.getByRole('alert')).toContainText(
    'Open the address workflow --web printed as it started',
  )
})
