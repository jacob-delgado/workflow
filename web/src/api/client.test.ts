import { client } from './generated/client.gen.ts'
import './client.ts'

test('makes API requests relative to the page origin, not the spec server URL', () => {
  // Act
  const baseUrl = client.getConfig().baseUrl

  // Assert
  // Empty, not the contract's absolute http://127.0.0.1:7000 — an absolute URL
  // would bypass the dev proxy and hit a CORS wall.
  expect(baseUrl).toBe('')
})
