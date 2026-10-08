import { RuleTester } from 'eslint'
import tseslint from 'typescript-eslint'
import { arrangeActAssert } from './arrange-act-assert.js'

// The marker check the web's tests answer to, as cmd/testshape is the Go
// tests': each case is a test body, valid when its markers hold and each
// Assert reaches an expect.
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
      name: 'a single cycle, its words in an ordinary comment under the bare marker',
      code: `test('t', async ({ page }) => {
  // Act
  // open the preview
  await page.click('x')

  // Assert
  expect(sent).toEqual([])
})`,
    },
    {
      name: 'a table test whose body marks its parts',
      code: `test.each(cases)('t $name', async ({ name }) => {
  // Act
  const said = say(name)

  // Assert
  expect(said).toBe(name)
})`,
    },
    {
      name: 'a test given a timeout after its body, whose body marks its parts',
      code: `test('t', async () => {
  // Act & Assert
  expect(1).toBe(1)
}, 10_000)`,
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
      name: 'a single Act labeled',
      code: `test('t', async ({ page }) => {
  // Act: open the preview
  await page.click('x')

  // Assert
  expect(sent).toEqual([])
})`,
      errors: [{ messageId: 'labelUnexpected' }],
    },
    {
      name: 'a single Assert labeled',
      code: `test('t', async ({ page }) => {
  // Act
  await page.click('x')

  // Assert: nothing is sent
  expect(sent).toEqual([])
})`,
      errors: [{ messageId: 'labelUnexpected' }],
    },
    {
      name: 'a single Act & Assert labeled',
      code: `test('t', async () => {
  // Act & Assert: one is one
  expect(1).toBe(1)
})`,
      errors: [{ messageId: 'labelUnexpected' }],
    },
    {
      name: 'an Arrange labeled, in a flow that labels every step',
      code: `test('t', async ({ page }) => {
  // Arrange: the home page
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
      errors: [{ messageId: 'labelUnexpected' }],
    },
    {
      name: 'a table test with no markers',
      code: `test.each(cases)('t $name', async ({ name }) => {
  expect(say(name)).toBe(name)
})`,
      errors: [{ messageId: 'missingMarkers' }],
    },
    {
      name: 'a test given a timeout after its body, with no markers',
      code: `test('t', async () => {
  expect(1).toBe(1)
}, 10_000)`,
      errors: [{ messageId: 'missingMarkers' }],
    },
    {
      name: 'a test given options after its body, with no markers',
      code: `test('t', async () => {
  expect(1).toBe(1)
}, { retry: 2 })`,
      errors: [{ messageId: 'missingMarkers' }],
    },
    {
      name: 'a table test given a timeout after its body, with no markers',
      code: `test.each(cases)('t $name', async ({ name }) => {
  expect(say(name)).toBe(name)
}, 10_000)`,
      errors: [{ messageId: 'missingMarkers' }],
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
