import { expect, test } from './fixtures.ts'
import { walkTabOrder } from './tabwalk.ts'

// What the Tab walk the layout specs lean on reports of a page of its own.

test('a control Tab reaches but the page never draws is out of view', async ({ page }) => {
  // Arrange
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

test('a walk within a dialog names the stop that leaves it', async ({ page }) => {
  // Arrange
  // An open dialog that is not modal, so it does not trap focus.
  await page.setContent(`
    <button>Outside</button>
    <dialog open><button>Inside</button></dialog>
  `)
  const dialog = page.getByRole('dialog')

  // Act
  const { reached, left } = await walkTabOrder(page, { within: dialog })

  // Assert
  expect({ reached, left }).toEqual({ reached: ['Inside'], left: ['Outside'] })
})

test('a walk within a dialog counts its controls alone, clipped by its box', async ({ page }) => {
  // Arrange
  await page.setContent(`
    <button>Behind</button>
    <dialog>
      <button>Shown</button>
      <div style="height: 2rem; overflow: clip">
        <div style="height: 4rem"></div>
        <button>Clipped</button>
      </div>
    </dialog>
    <script>document.querySelector('dialog').showModal()</script>
  `)
  const dialog = page.getByRole('dialog')

  // Act
  const { missed, hidden, left } = await walkTabOrder(page, { within: dialog })

  // Assert
  expect({ missed, hidden, left }).toEqual({
    missed: [],
    hidden: ['Clipped (0% in view)'],
    left: [],
  })
})

test('a walk that starts partway round comes back in at the top of the page', async ({ page }) => {
  // Arrange
  await page.setContent(`
    <button>Top</button>
    <button>Middle</button>
    <button>Last</button>
  `)
  await page.getByRole('button', { name: 'Middle' }).focus()

  // Act
  const { reached, missed } = await walkTabOrder(page)

  // Assert
  expect({ reached, missed }).toEqual({ reached: ['Top', 'Middle', 'Last'], missed: [] })
})

test('a lap past a control that keeps Tab to itself never closes', async ({ page }) => {
  // Arrange
  await page.setContent(`
    <button>Before</button>
    <button>Keeper</button>
    <script>
      document.querySelector('button:last-of-type').addEventListener('keydown', (event) => {
        if (event.key === 'Tab') event.preventDefault()
      })
    </script>
  `)

  // Act & Assert
  await expect(walkTabOrder(page)).rejects.toThrow('did not come back round')
})
