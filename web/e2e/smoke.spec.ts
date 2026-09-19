import { expect, test } from '@playwright/test'

test('serves the cockpit shell', async ({ page }) => {
  // Act
  await page.goto('/')

  // Assert
  await expect(page.getByRole('heading', { level: 1, name: /workflow/i })).toBeVisible()
})
