import { expect, test } from './fixtures.ts'

// What every hermetic spec's page meets before its own routes: the API it
// does not answer is refused here, never sent on to a server.

test('a read no spec answers is refused as a route the server does not have', async ({ page }) => {
  // Arrange
  await page.goto('/')

  // Act
  const answer = await page.evaluate(async () => {
    const response = await fetch('/api/health')

    return {
      status: response.status,
      type: response.headers.get('content-type'),
      code: ((await response.json()) as { code?: string }).code,
    }
  })

  // Assert
  expect(answer).toEqual({ status: 404, type: 'application/problem+json', code: 'not_found' })
})
