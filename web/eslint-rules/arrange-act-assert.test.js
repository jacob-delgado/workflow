import { RuleTester } from 'eslint'
import tseslint from 'typescript-eslint'
import { arrangeActAssert } from './arrange-act-assert.js'

// The marker check e2e specs answer to, as cmd/testshape is the Go tests':
// each case is a spec body, valid when its markers hold and each Assert
// reaches an expect.
const tester = new RuleTester({ languageOptions: { parser: tseslint.parser } })

tester.run('arrange-act-assert', arrangeActAssert, {
  valid: [
    {
      name: 'an Arrange, an Act and an Assert that expects',
      code: `test('t', async ({ page }) => {
  // Arrange
  await page.goto('/')

  // Act
  await page.click('x')

  // Assert
  await expect(page).toHaveTitle('t')
})`,
    },
    {
      name: 'an Act & Assert through a helper named for expecting',
      code: `test('t', async ({ page }) => {
  // Act & Assert
  await expectReachableAndClean(page)
})`,
    },
    {
      name: 'a flow that labels every step',
      code: `test('t', async ({ page }) => {
  // Arrange
  await page.goto('/')

  // Act: open the preview
  await page.click('x')

  // Assert: nothing is sent yet
  expect(sent).toEqual([])

  // Act: confirm
  await page.click('y')

  // Assert: it is sent
  expect(sent).toEqual(['y'])
})`,
    },
    {
      name: 'an Arrange that waits on the page',
      code: `test('t', async ({ page }) => {
  // Arrange
  await page.goto('/')
  await expect(page.getByRole('main')).toBeVisible()

  // Act & Assert
  expect(1).toBe(1)
})`,
    },
    {
      name: 'a hook, which is not a test',
      code: `test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
})`,
    },
  ],
  invalid: [
    {
      name: 'no markers',
      code: `test('t', async () => {
  expect(1).toBe(1)
})`,
      errors: [{ messageId: 'missingMarkers' }],
    },
    {
      name: 'an Assert that reaches no expect',
      code: `test('t', async ({ page }) => {
  // Act
  await page.click('x')

  // Assert
  await page.getByRole('status').isVisible()
})`,
      errors: [{ messageId: 'assertWithoutExpect' }],
    },
    {
      name: 'code before the first marker',
      code: `test('t', async ({ page }) => {
  await page.goto('/')

  // Act & Assert
  expect(1).toBe(1)
})`,
      errors: [{ messageId: 'codeBeforeMarker' }],
    },
    {
      name: 'a marker written in another case',
      code: `test('t', async () => {
  // act
  const one = 1

  // Assert
  expect(one).toBe(1)
})`,
      errors: [{ messageId: 'malformedMarker' }],
    },
    {
      name: 'an Assert before any Act',
      code: `test('t', async () => {
  // Assert
  expect(1).toBe(1)
})`,
      errors: [{ messageId: 'markerOrder' }],
    },
    {
      name: 'a flow with a step unlabeled',
      code: `test('t', async ({ page }) => {
  // Act: open the preview
  await page.click('x')

  // Assert
  expect(sent).toEqual([])

  // Act: confirm
  await page.click('y')

  // Assert: it is sent
  expect(sent).toEqual(['y'])
})`,
      errors: [{ messageId: 'labelRequired' }],
    },
    {
      name: 'a section with nothing in it',
      code: `test('t', async () => {
  // Arrange

  // Act & Assert
  expect(1).toBe(1)
})`,
      errors: [{ messageId: 'emptySection' }],
    },
    {
      name: 'a value checked in an Arrange',
      code: `test('t', async ({ page }) => {
  // Arrange
  await page.click('x')
  expect(sent).toEqual([])

  // Act
  await page.click('y')

  // Assert
  expect(sent).toEqual(['y'])
})`,
      errors: [{ messageId: 'checkOutsideAssert' }],
    },
    {
      name: 'a marker inside a statement',
      code: `test('t', async ({ page }) => {
  // Act
  await page.route('**', (route) => {
    // Assert
    return route.fulfill({})
  })

  // Assert
  expect(1).toBe(1)
})`,
      errors: [{ messageId: 'markerPlacement' }],
    },
  ],
})
