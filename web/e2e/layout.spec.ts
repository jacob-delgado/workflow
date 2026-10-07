import { expect, test, type Page } from '@playwright/test'
import type { Issue, PullRequestDraft, Snapshot } from '../src/api/generated/types.gen.ts'
import {
  height,
  openCockpit,
  openFirstRun,
  openSection,
  sectionNames,
  themes,
  widths,
} from './cockpit.ts'
import { axeViolations, pageScrolls, sidewaysScrollers, streams, walkTabOrder } from './tabwalk.ts'

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
          // Act: open the section, and Tab once round it.
          await openSection(page, name)
          const { missed, hidden } = await walkTabOrder(page)

          // Assert: nothing scrolls sideways, nor the page down; Tab reaches
          // every drawn control, each in view as it has focus; and axe finds
          // nothing.
          expect(await page.evaluate(sidewaysScrollers), `${name}: scrolls sideways`).toEqual([])
          expect(await page.evaluate(pageScrolls), `${name}: the page scrolls`).toBe(false)
          expect(missed, `${name}: never reached by Tab`).toEqual([])
          expect(hidden, `${name}: out of view with focus`).toEqual([])
          expect(await axeViolations(page), `${name}: axe`).toBe('')
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

      // Act: Tab once round the page.
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert: nothing scrolls sideways, nor the page down; Tab reaches every
      // question and the write, each in view as it has focus; and axe finds
      // nothing.
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
      expect(reached, 'reached by Tab').toEqual(
        expect.arrayContaining(['Write ~/src/api/.workflow.json']),
      )
      // The home directory's radio button is reached by the arrow keys from
      // the one chosen, the one stop Tab passes by.
      expect(missed, 'never reached by Tab').toEqual([
        '~/.workflow.jsonYour home directory: it applies everywhere.',
      ])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
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

test(
  'a wide window sets each issue summary in the list on one line',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await openCockpit(page, { width: 1440, height }, 'dark')

    // Act: how many lines each row's summary, its last line, is set on.
    const lines = await page
      .getByRole('list', { name: 'Issues' })
      .getByRole('button')
      .filter({ hasNotText: 'Switch branch' })
      .evaluateAll((rows) =>
        rows.map((row) => {
          const summary = row.lastElementChild ?? row

          return Math.round(
            summary.getBoundingClientRect().height /
              parseFloat(getComputedStyle(summary).lineHeight),
          )
        }),
      )

    // Assert
    expect(lines.length).toBeGreaterThan(1)
    expect(lines).toEqual(lines.map(() => 1))
  },
)

for (const width of [640, 1440]) {
  test(
    `every issue row ends at one right edge at ${String(width)} px, with or without Switch branch`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width, height }, 'dark')
      const list = page.getByRole('list', { name: 'Issues' })
      await expect(list.getByRole('button', { name: /^Switch branch/ }).first()).toBeVisible()

      // Act
      const edges = await list
        .getByRole('button')
        .filter({ hasNotText: 'Switch branch' })
        .evaluateAll((rows) => rows.map((row) => Math.round(row.getBoundingClientRect().right)))

      // Assert
      expect(new Set(edges).size, `right edges ${edges.join(', ')}`).toBe(1)
    },
  )
}

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

          // Act: open the step, and Tab once round the page.
          await page.getByRole('button', { name: opener }).click()
          await expect(page.getByRole('group', { name: group })).toBeVisible()
          const { reached, missed, hidden } = await walkTabOrder(page)

          // Assert: nothing scrolls sideways, nor the page down; Tab reaches
          // the step's controls and every other drawn one, each in view as it
          // has focus; and axe finds nothing.
          expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
          expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
          expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(adds))
          expect(missed, 'never reached by Tab').toEqual([])
          expect(hidden, 'out of view with focus').toEqual([])
          expect(await axeViolations(page), 'axe').toBe('')
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

const issuesTotal = 12

// streamedIssue is one of the issues the stream carries, or a later page adds.
function streamedIssue(number: number): Issue {
  return {
    key: `PROJ-${String(number)}`,
    tracker: 'jira',
    summary: `Issue number ${String(number)} of the view`,
    status: 'To Do',
    status_category: 'new',
    type: 'Task',
  }
}

// A stream carrying the first eight of the view's twelve issues, so the list
// offers to load more.
const pagedSnapshot = {
  issues: {
    total: issuesTotal,
    start_at: 0,
    unavailable: [],
    issues: [1, 2, 3, 4, 5, 6, 7, 8].map(streamedIssue),
  },
  branch: {
    name: '',
    issue_link: '',
    detached: false,
    head: '',
    upstream: '',
    push_remote: '',
    ahead: 0,
    behind: 0,
    base: '',
    commits: [],
  },
  changes: { changes: [] },
  review: { found: false, announced: false },
  messaging: { service: 'Slack', configured: false, channel: '', channels: [], author: '' },
  branches: [],
  commit_types: ['feat', 'fix'],
  subject_limit: 72,
  suggested_scope: '',
  hooks_unmanaged: 0,
  tasks: { available: true, reason: '', linked: [] },
  here: '/home/ana/src/api',
} satisfies Snapshot

test('a loaded page hands focus to its first issue, in view in the list, at 640 px', async ({
  page,
}) => {
  // Arrange: the stream's eight issues, and the next page answered here, in a
  // window where the list scrolls in its pane.
  await streams(page, pagedSnapshot)
  await page.route(/\/api\/issues\?/, (route) =>
    route.fulfill({
      json: {
        total: issuesTotal,
        start_at: 8,
        unavailable: [],
        issues: [9, 10, 11, 12].map(streamedIssue),
      },
    }),
  )
  await page.setViewportSize({ width: 640, height: 700 })
  await page.goto('/')

  // Act
  await page.getByRole('button', { name: 'Load more' }).click()

  // Assert: the first issue the page added has focus, and is in view.
  const added = page.getByRole('button', { name: /^PROJ-9/ })
  await expect(added).toBeFocused()
  await expect(added).toBeInViewport({ ratio: 1 })
})

// A branch and a file whose names each hold a word wider than the content.
const unbrokenSnapshot = {
  ...pagedSnapshot,
  branch: {
    ...pagedSnapshot.branch,
    name: 'fix/PROJ-1',
    issue_link: '',
    head: 'abc1234',
    upstream: 'origin/redact_every_authorization_header_before_the_request_log_writes_it',
    base: 'origin/main',
  },
  changes: {
    changes: [
      {
        path: 'internal/tui/testdata/TestScreenDrawsEveryPaneAtTheNarrowestWidthItAllows.golden',
        kind: 'modified',
        staged: false,
        has_unstaged: true,
        conflicted: false,
      },
    ],
  },
  here: '/home/ana/src/api',
} satisfies Snapshot

// A source build's health: its version is a commit marked dirty, the widest
// the header draws.
const sourceBuild = {
  version: 'ddbb935d6c04-dirty',
  dry_run: false,
  forge_noun: 'pull request',
  forge_sigil: '#',
}

// widthDrawn is how wide an element is drawn.
function widthDrawn(element: Element): number {
  return element.getBoundingClientRect().width
}

// termsSlack is how much wider the terms' column is than the widest term in it.
function termsSlack(terms: Element[]): number {
  const column = Math.max(...terms.map((term) => term.getBoundingClientRect().width))
  const words = terms.map((term) => {
    const range = document.createRange()
    range.selectNodeContents(term)

    return range.getBoundingClientRect().width
  })

  return column - Math.max(...words)
}

test('the branch and the header fit 320 px, wrapping a name wider than the content', async ({
  page,
}) => {
  // Arrange: the stream's branch and working tree, and a source build's
  // version in the header, at the narrowest width a page must reflow to.
  await streams(page, unbrokenSnapshot)
  await page.route('**/api/health', (route) => route.fulfill({ json: sourceBuild }))
  await page.setViewportSize({ width: 320, height })
  await page.goto('/')

  // Act
  await openSection(page, 'Branch')

  // Assert: nothing scrolls sideways; the file's path takes a line of its
  // own; and the terms' column is as wide as its widest term.
  const path = page.getByText(/^internal\/tui\/testdata/)
  const row = page.getByRole('listitem').filter({ has: path })
  await expect(path).toBeVisible()
  expect(await page.evaluate(sidewaysScrollers)).toEqual([])
  expect(await path.evaluate(widthDrawn), 'the path takes the row').toBeGreaterThanOrEqual(
    (await row.evaluate(widthDrawn)) - 1,
  )
  expect(await page.getByRole('term').evaluateAll(termsSlack), 'the terms column').toBeLessThan(1)
})

// The pull request the branch's work composes, as the draft read answers it.
const pullDraft = {
  title: 'fix: redact tokens before they reach the request log',
  body: 'Redacts the Authorization header.',
  base: 'main',
  head: 'fix/PROJ-1',
  draft: false,
  needs_push: false,
  reviewers: ['ana', 'acme/control-plane'],
  templates: [],
  template: '',
} satisfies PullRequestDraft

// The pull request form's fields and buttons, named as a walk names them.
const formControls = [
  'Title',
  'Base branch',
  'Reviewers',
  'Assignees',
  'Labels',
  'Description',
  'Open as a draft',
  'Cancel',
  'Open pull request',
]

for (const width of widths) {
  test(`the pull request form fits ${String(width)} px, each field reached in view`, async ({
    page,
  }) => {
    // Arrange: a stream with no pull request yet, and the draft answered here.
    await streams(page, pagedSnapshot)
    await page.route('**/api/pull-request/draft', (route) => route.fulfill({ json: pullDraft }))
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.setViewportSize({ width, height })
    await page.goto('/')
    await openSection(page, 'Review')

    // Act: compose the pull request, and Tab once round the page.
    await page.getByRole('button', { name: 'Open a pull request' }).click()
    await expect(page.getByRole('form', { name: 'Open a pull request' })).toBeVisible()
    const { reached, missed, hidden } = await walkTabOrder(page)

    // Assert: nothing scrolls sideways, nor the page down; and Tab reaches
    // each of the form's fields and every other drawn control, in view.
    expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
    expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
    expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(formControls))
    expect(missed, 'never reached by Tab').toEqual([])
    expect(hidden, 'out of view with focus').toEqual([])
  })
}

for (const width of widths) {
  test(`a refused write fits ${String(width)} px, every control reached in view`, async ({
    page,
  }) => {
    // Arrange: the stream's working tree, and a stage the server refuses.
    await streams(page, unbrokenSnapshot)
    await page.route('**/api/stage', (route) => route.fulfill({ status: 500 }))
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.setViewportSize({ width, height })
    await page.goto('/')
    await openSection(page, 'Branch')

    // Act: stage the file, let the refusal land, and Tab once round the page.
    await page.getByRole('button', { name: /^Stage internal\/tui/ }).click()
    await expect(page.getByRole('alert')).toBeVisible()
    const { missed, hidden } = await walkTabOrder(page)

    // Assert: nothing scrolls sideways, nor the page down; and Tab reaches
    // every drawn control beside the refusal, each in view.
    expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
    expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
    expect(missed, 'never reached by Tab').toEqual([])
    expect(hidden, 'out of view with focus').toEqual([])
  })
}
