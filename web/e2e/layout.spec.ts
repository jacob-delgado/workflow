import type { Page } from '@playwright/test'
import {
  height,
  openCockpit,
  openFirstRun,
  openSection,
  sectionNames,
  themes,
  widths,
} from './support/cockpit.ts'
import { expectReachableAndClean, pageScrolls } from './support/reachable.ts'
import { expect, test } from './support/fixtures.ts'

// The populated cockpit at a narrow, a middling and a wide window, in both
// themes: nothing scrolls sideways, the page does not scroll at all, every
// control the keyboard can reach is in view once it has focus, and axe finds
// nothing.

for (const theme of themes) {
  for (const width of widths) {
    test(
      `every section fits ${String(width)} px in the ${theme} theme, reachable and clean`,
      { tag: '@populated' },
      async ({ page }) => {
        // Nine sections, each walked and scanned by axe, are nine tests' work
        // in one: the budget for one runs out on a busy machine.
        test.slow()

        // Arrange: the populated cockpit at this width, in this theme.
        await openCockpit(page, { width, height }, theme)

        for (const name of sectionNames) {
          // Act: open the section.
          await openSection(page, name)

          // Assert: it is reachable and clean.
          await expectReachableAndClean(page)
        }
      },
    )
  }
}

// Settings on a server with no configuration file, where it sets one up, at
// each width in both themes.
for (const theme of themes) {
  for (const width of widths) {
    test(`the first-run setup fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange: Settings with no file, at this width, in this theme.
      await openFirstRun(page, { width, height }, theme)

      // Act & Assert: Tab reaches every question and the write, passing by
      // the home directory's radio button, which the arrow keys reach from the
      // one chosen.
      await expectReachableAndClean(page, {
        reaches: ['Write ~/src/api/.workflow.json'],
        passedBy: ['~/.workflow.jsonYour home directory: it applies everywhere.'],
      })
    })
  }
}

// widestContent is the width of the widest field, paragraph or list a section
// draws: the measure its content is set at.
async function widestContent(page: Page): Promise<number> {
  const main = page.getByRole('main')
  const widths = await Promise.all(
    (['textbox', 'combobox', 'paragraph', 'list'] as const).map((role) =>
      main
        .getByRole(role)
        .evaluateAll((parts) => parts.map((part) => part.getBoundingClientRect().width)),
    ),
  )

  return Math.max(...widths.flat())
}

test(
  "every section sets its content at the Branch section's measure in a wide window",
  { tag: '@populated' },
  async ({ page }) => {
    // Nine sections, each opened and measured, in one test.
    test.slow()

    // Arrange: the populated cockpit at the wide width, and the measure
    // Branch's commit form is set at.
    await openCockpit(page, { width: 1440, height }, 'dark')
    await openSection(page, 'Branch')
    const measure = await widestContent(page)

    for (const name of sectionNames) {
      // Act: open the section.
      await openSection(page, name)

      // Assert: nothing in it is set wider than Branch's form.
      expect(await widestContent(page), `${name}: wider than Branch`).toBeLessThanOrEqual(
        measure + 1,
      )
    }
  },
)

// The steps a click opens on the populated build before a write goes out, and
// the controls each adds, which Tab must reach in view.
const confirmSteps = [
  {
    step: 'push confirmation',
    section: 'Branch',
    opener: 'Push branch',
    group: /^Push /,
    adds: ['Cancel', 'Push'],
  },
  {
    step: 'discard confirmation',
    section: 'Branch',
    opener: 'Discard internal/config/redact.go…',
    group: 'Discard the changes to internal/config/redact.go?',
    adds: ['Cancel', 'Discard'],
  },
  {
    step: 'announcement preview',
    section: 'Slack',
    opener: 'Announce to Slack',
    group: 'Announcement preview',
    // The mockup's announcement tags: an owner to link, and a group to check.
    adds: [
      'Channel',
      'Slack user for ben',
      'ben is not on Slack',
      '@api-reviewers',
      'Cancel',
      'Announce now',
    ],
  },
  {
    step: 'summary preview',
    section: 'Summary',
    opener: 'Post…',
    group: 'Summary preview',
    adds: ['Edit', 'Channel', 'Cancel', 'Post'],
  },
  {
    step: 'forget confirmation in People and groups',
    section: 'Settings',
    opener: 'Forget carla…',
    group: 'Forget carla?',
    adds: ['Cancel', 'Forget'],
  },
  {
    step: 'credential removal in Settings',
    section: 'Settings',
    opener: 'Remove the Jira token…',
    group: 'Remove the Jira token from the file?',
    adds: ['Cancel', 'Remove'],
  },
  {
    step: 'remove confirmation in Local data',
    section: 'Settings',
    opener: 'Remove cache…',
    group: 'Remove workflow.db?',
    adds: ['Cancel', 'Remove'],
  },
]

for (const theme of themes) {
  for (const width of widths) {
    for (const { step, section, opener, group, adds } of confirmSteps) {
      test(
        `the ${step} fits ${String(width)} px in the ${theme} theme, reachable and clean`,
        { tag: '@populated' },
        async ({ page }) => {
          // Arrange: the populated cockpit at this width, on the step's section.
          await openCockpit(page, { width, height }, theme)
          await openSection(page, section)

          // Act: open the step.
          await page.getByRole('button', { name: opener }).click()
          await expect(page.getByRole('group', { name: group })).toBeVisible()

          // Assert: Tab reaches the step's controls, and it is clean.
          await expectReachableAndClean(page, { reaches: adds })
        },
      )
    }
  }
}

// From md (768 px) the rail names each section beside its icon; below it, the
// rail keeps only the icons, and each name stays the button's accessible name,
// and its title, which a pointer resting on the icon shows.
const railCases = [
  { width: 640, named: false },
  { width: 1024, named: true },
  { width: 1440, named: true },
]

for (const { width, named } of railCases) {
  test(
    `the rail ${named ? 'names its sections' : 'keeps only its icons'} at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await page.setViewportSize({ width, height })
      await page.goto('/')
      const rail = page.getByRole('navigation', { name: 'Sections' })
      const buttons = sectionNames.map((name) => rail.getByRole('button', { name, exact: true }))

      // Act: measure each name as it is drawn.
      const drawn = await Promise.all(
        buttons.map(async (button, index) => {
          const name = await button.getByText(sectionNames[index], { exact: true }).boundingBox()

          return (name?.width ?? 0) > 1
        }),
      )

      // Assert: each button keeps its name, as its title too, drawn only from
      // md up.
      for (const [index, button] of buttons.entries()) {
        await expect(button).toBeVisible()
        await expect(button).toHaveAttribute('title', sectionNames[index])
      }
      expect(drawn).toEqual(sectionNames.map(() => named))
    },
  )
}

// The header holds still while a section scrolls beneath it: the page itself
// never scrolls, the content does.
for (const width of widths) {
  test(
    `the header stays in view as a long section scrolls at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange: the settings, longer than the window.
      await page.setViewportSize({ width, height })
      await page.goto('/')
      await openSection(page, 'Settings')
      const save = page.getByRole('button', { name: 'Save changes' })

      // Act: take the form's last control into view.
      await save.focus()

      // Assert: it is in view, and so is the header above it.
      await expect(save).toBeInViewport()
      await expect(page.getByRole('button', { name: /change theme/i })).toBeInViewport()
    },
  )
}

// In a window shorter than the rail, the rail scrolls itself, not the page:
// the header stays in view as the rail's last section takes focus.
test(
  'the rail scrolls itself in a window shorter than it',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await page.setViewportSize({ width: 640, height: 240 })
    await page.goto('/')
    const rail = page.getByRole('navigation', { name: 'Sections' })
    const settings = rail.getByRole('button', { name: 'Settings', exact: true })

    // Act
    await settings.focus()

    // Assert
    await expect(settings).toBeInViewport()
    await expect(page.getByRole('button', { name: /change theme/i })).toBeInViewport()
    expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
  },
)

// The skip link is the first stop, drawn as it has focus, and it hands focus to
// the content.
for (const width of widths) {
  test(
    `the skip link takes focus to the content at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await page.setViewportSize({ width, height })
      await page.goto('/')
      const skip = page.getByRole('link', { name: 'Skip to content' })

      // Act: the first Tab
      await page.keyboard.press('Tab')

      // Assert: it lands on the skip link, drawn in view
      await expect(skip).toBeFocused()
      await expect(skip).toBeInViewport({ ratio: 1 })

      // Act: follow it
      await page.keyboard.press('Enter')

      // Assert: the content has focus
      await expect(page.getByRole('main')).toBeFocused()
    },
  )
}

// A section opens at its top, wherever the last one was scrolled to, so the
// content that takes focus as it opens shows its heading.
for (const width of widths) {
  test(
    `a section opens at its top at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange: the settings, scrolled to their end.
      await page.setViewportSize({ width, height })
      await page.goto('/')
      await openSection(page, 'Settings')
      await page.getByRole('button', { name: 'Save changes' }).focus()

      // Act
      await openSection(page, 'Branch')

      // Assert
      await expect(page.getByRole('heading', { level: 1, name: 'Branch' })).toBeInViewport({
        ratio: 1,
      })
    },
  )
}
