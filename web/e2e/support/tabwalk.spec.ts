import { expect, test } from './fixtures.ts'
import { walkTabOrder } from './tabwalk.ts'

// What the Tab walk the layout specs lean on reports of a page of its own.

test('a control Tab reaches but the page never draws is out of view', async ({ page }) => {
  // Arrange: a button between two others, drawn at no size at all.
  await page.setContent(`
    <button>Before</button>
    <button style="width: 0; height: 0; padding: 0; border: 0; overflow: hidden">Undrawn</button>
    <button>After</button>
  `)

  // Act
  const { hidden } = await walkTabOrder(page)

  // Assert
  expect(hidden).toEqual(['Undrawn (0% in view)'])
})
