import { expect, test } from '@playwright/test'

test('serves the cockpit shell', async ({ page }) => {
  // Act
  await page.goto('/')

  // Assert
  await expect(page.getByRole('navigation', { name: /sections/i })).toBeVisible()
  await expect(page.getByRole('heading', { level: 1, name: /issues/i })).toBeVisible()
})
