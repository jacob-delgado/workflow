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

test('a guard handing focus back to the only control of a dialog keeps the walk within it', async ({
  page,
}) => {
  // Arrange
  // Focus guards as Base UI draws them round a modal dialog: hidden from
  // assistive technology, each handing focus back in on the next frame.
  await page.setContent(`
    <button>Outside</button>
    <span class="guard" tabindex="0" aria-hidden="true"></span>
    <div role="dialog" aria-label="Notice"><button>Only</button></div>
    <span class="guard" tabindex="0" aria-hidden="true"></span>
    <style>
      .guard {
        position: fixed; top: 0; left: 0; width: 1px; height: 1px;
        overflow: hidden; clip-path: inset(50%);
      }
    </style>
    <script>
      for (const guard of document.querySelectorAll('.guard')) {
        guard.addEventListener('focus', () => {
          requestAnimationFrame(() => document.querySelector('[role=dialog] button').focus())
        })
      }
    </script>
  `)
  await page.getByRole('button', { name: 'Only' }).focus()

  // Act
  const { reached, left } = await walkTabOrder(page, { within: page.getByRole('dialog') })

  // Assert
  expect({ reached, left }).toEqual({ reached: ['Only'], left: [] })
})

test('a page that keeps its focus events from the walk is still walked to its end', async ({
  page,
}) => {
  // Arrange
  // Focus moves with no focus event the walk can see, as it does into a frame.
  await page.setContent(`
    <button>Before</button>
    <button>After</button>
    <script>
      window.addEventListener('focusin', (event) => event.stopImmediatePropagation(), true)
    </script>
  `)

  // Act
  const { reached, missed } = await walkTabOrder(page)

  // Assert
  expect({ reached, missed }).toEqual({ reached: ['Before', 'After'], missed: [] })
})

test('a field the browser reveals only at its next rendering update is in view as it has focus', async ({
  page,
}) => {
  // Arrange
  // As WebKit does, the field comes into view only at the rendering update
  // after Tab gives it focus; the page asks for no frame itself, so the field
  // is revealed only once the walk waits for one.
  await page.setContent(`
    <button>Before</button>
    <div style="height: 100px; overflow-y: auto">
      <div style="height: 300px"></div>
      <label>Late <input id="late"></label>
    </div>
    <script>
      const late = document.getElementById('late')
      let pending = false
      const frame = window.requestAnimationFrame.bind(window)
      window.requestAnimationFrame = (callback) =>
        frame((time) => {
          if (pending) {
            pending = false
            late.scrollIntoView({ block: 'nearest' })
          }
          callback(time)
        })
      window.addEventListener('keydown', (event) => {
        if (event.key === 'Tab' && !event.shiftKey && document.activeElement?.textContent === 'Before') {
          event.preventDefault()
          late.focus({ preventScroll: true })
          pending = true
        }
      }, true)
    </script>
  `)

  // Act
  const { hidden } = await walkTabOrder(page)

  // Assert
  expect(hidden).toEqual([])
})
