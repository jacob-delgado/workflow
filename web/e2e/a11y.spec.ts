import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

// Scan the resting state, not mid-animation frames: reduced motion collapses
// transitions to instant, so axe never samples a half-faded element (whose
// transient blended colors are a false contrast failure).
test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
})

test('the cockpit has no accessibility violations', async ({ page }) => {
  // Arrange
  await page.goto('/')

  // Act
  const { violations } = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()

  // Assert
  const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
  expect(violations, `a11y violations: ${summary}`).toEqual([])
})
