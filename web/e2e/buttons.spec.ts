import type { Locator, Page } from '@playwright/test'
import { height, openCockpit, openSection, themes } from './support/cockpit.ts'
import { expect, test } from './support/fixtures.ts'

// A section's one outward act is drawn at one size wherever it stands, and
// every control beside it at the other: a reader learns the two sizes once.

// The populated cockpit's primary acts, each with the section that offers it.
const primaryActs = [
  ['Branch', 'Commit staged changes'],
  ['Slack', 'Announce to Slack'],
  ['Settings', 'Save changes'],
] as const

// heightDrawn is how tall a control is drawn.
function heightDrawn(control: Locator): Promise<number> {
  return control.evaluate((element) => element.getBoundingClientRect().height)
}

// primaryHeights opens each section in turn and says how tall its primary act
// is drawn, by the act's name.
async function primaryHeights(page: Page): Promise<Record<string, number>> {
  const drawn: Record<string, number> = {}
  for (const [section, name] of primaryActs) {
    await openSection(page, section)
    drawn[name] = await heightDrawn(page.getByRole('button', { name }))
  }

  return drawn
}

test('every primary act is drawn at one height', { tag: '@populated' }, async ({ page }) => {
  // Arrange
  await openCockpit(page, { width: 1024, height }, 'dark')

  // Act
  const drawn = await primaryHeights(page)

  // Assert
  expect(new Set(Object.values(drawn)).size, JSON.stringify(drawn)).toBe(1)
})

test(
  "the push confirmation draws its buttons at the Branch section's two sizes",
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await openCockpit(page, { width: 1024, height }, 'dark')
    await openSection(page, 'Branch')
    const primary = await heightDrawn(page.getByRole('button', { name: 'Commit staged changes' }))
    const opener = page.getByRole('button', { name: 'Push branch' })
    const secondary = await heightDrawn(opener)

    // Act
    await opener.click()

    // Assert
    const confirmation = page.getByRole('group', { name: /^Push .* to origin\?$/ })
    const push = confirmation.getByRole('button', { name: 'Push', exact: true })
    expect(await heightDrawn(push), 'Push').toBe(primary)
    expect(await heightDrawn(confirmation.getByRole('button', { name: 'Cancel' })), 'Cancel').toBe(
      secondary,
    )
  },
)

// A box-shadow layer, as the browser computes it: its color and its spread.
interface ShadowLayer {
  color: string
  spread: number
}

// focusDrawing is how a control draws keyboard focus once Tab reaches it — the
// layers of its computed box-shadow — and the page color around it. It steps
// back and Tabs onto the control, since only focus a key moves is drawn.
async function focusDrawing(control: Locator): Promise<{ layers: ShadowLayer[]; page: string }> {
  await control.focus()
  await control.press('Shift+Tab')
  await control.page().keyboard.press('Tab')
  await expect(control, 'Tab returns to the control').toBeFocused()

  return control.evaluate((element) => {
    const shadow = getComputedStyle(element).boxShadow
    const layers = [...shadow.matchAll(/(rgba?\([^)]*\))\s+0px 0px 0px (\d+)px/g)].map(
      ([, color = '', spread = '0']) => ({ color, spread: Number(spread) }),
    )

    return { layers, page: getComputedStyle(document.body).backgroundColor }
  })
}

for (const theme of themes) {
  test(
    `a primary act's focus ring stands apart from its fill in the ${theme} theme`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width: 1024, height }, theme)
      await openSection(page, 'Settings')

      // Act
      const { layers, page: pageColor } = await focusDrawing(
        page.getByRole('button', { name: 'Save changes' }),
      )

      // Assert
      const gap = layers.find((layer) => layer.color === pageColor && layer.spread > 0)
      expect(gap, JSON.stringify({ layers, pageColor })).toBeDefined()
      const ring = layers.find((layer) => layer.spread > (gap?.spread ?? 0))
      expect(ring, JSON.stringify(layers)).toBeDefined()
    },
  )
}
