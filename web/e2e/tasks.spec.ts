import { expect, test, type Locator, type Page } from '@playwright/test'
import type { Snapshot, Task, TaskList, TasksSummary } from '../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from './cockpit.ts'
import { axeViolations, pageScrolls, sidewaysScrollers, streams, walkTabOrder } from './tabwalk.ts'

// Your Taskwarrior tasks as the browser draws them, against answers given
// here: where a refusal sits against its row's buttons, how the header's task
// chip gives way in a narrow window, where a mark sits beside a description
// that wraps, the hue a task's mark is drawn in, and a refusal's line breaks.

// tracking is a task Taskwarrior holds for PROJ-1, pending and not started.
const tracking = {
  uuid: '5f1d7a3c-9b2e-4c8d-a6f0-3e1b2c4d5a6f',
  id: 1,
  description: 'PROJ-1: Refuse to start when the config names an unknown forge',
  status: 'pending',
  project: '',
  priority: '',
  tags: [],
  entry: '2026-09-20T10:00:00Z',
  modified: '2026-09-20T10:00:00Z',
  urgency: 8.2,
  annotations: [],
  issue_key: 'PROJ-1',
  issue_url: '',
} satisfies Task

// started is the same task, started.
const started = { ...tracking, start: '2026-09-28T09:00:00Z' } satisfies Task

// withTasks is a stream frame of one issue, PROJ-1, and of your tasks as given.
function withTasks(tasks: TasksSummary): Snapshot {
  return {
    here: '/home/ana/src/api',
    issues: {
      total: 1,
      start_at: 0,
      unavailable: [],
      issues: [
        {
          key: 'PROJ-1',
          tracker: 'jira',
          summary: 'Refuse an unknown forge',
          status: 'To Do',
          status_category: 'new',
          type: 'Task',
        },
      ],
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
    tasks,
  }
}

// listOf is the task list as the server answers a read of it.
function listOf(...tasks: Task[]): TaskList {
  return { available: true, reason: '', context: '', sync_available: false, said: '', tasks }
}

// refused is a problem answer, as the server gives a read or a write it
// refuses.
function refused(status: number, code: string, detail: string) {
  return {
    status,
    contentType: 'application/problem+json',
    body: JSON.stringify({ title: 'Refused', status, detail, code }),
  }
}

// Edges are where an element is drawn, in pixels from the window's edges.
interface Edges {
  top: number
  bottom: number
  left: number
  right: number
}

// edgesOf is where an element is drawn.
function edgesOf(locator: Locator): Promise<Edges> {
  return locator.evaluate((element) => {
    const { top, bottom, left, right } = element.getBoundingClientRect()

    return { top, bottom, left, right }
  })
}

// middleOf is the height halfway down an element.
function middleOf(edges: Edges): number {
  return (edges.top + edges.bottom) / 2
}

test('a refused start sits below its row of buttons, not between them', async ({ page }) => {
  // Arrange: a pending task, and a start of it Taskwarrior makes nothing of.
  await streams(page, withTasks({ available: true, reason: '', linked: [tracking] }))
  await page.route('**/api/tasks', (route) => route.fulfill({ json: listOf(tracking) }))
  await page.route('**/api/tasks/*/start', (route) =>
    route.fulfill(refused(409, 'conflict', 'the task is already in that state')),
  )
  await page.setViewportSize({ width: 1024, height })
  await page.goto('/')
  await openSection(page, 'Tasks')

  // Act
  await page.getByRole('button', { name: 'Start', exact: true }).click()
  await expect(page.getByRole('alert')).toBeVisible()

  // Assert: Start and Done share a row, and the refusal sits below both.
  const start = await edgesOf(page.getByRole('button', { name: 'Start', exact: true }))
  const done = await edgesOf(page.getByRole('button', { name: 'Mark done…', exact: true }))
  const refusal = await edgesOf(page.getByRole('alert'))
  expect(done.top, 'Mark done beside Start').toBe(start.top)
  expect(refusal.top, 'the refusal below both').toBeGreaterThanOrEqual(done.bottom)
})

test('the header keeps one row beside a long started task, at 640 px', async ({ page }) => {
  // Arrange: a started task whose description is longer than the header has
  // room for.
  await streams(
    page,
    withTasks({ available: true, reason: '', active: started, linked: [started] }),
  )
  await page.setViewportSize({ width: 640, height })

  // Act
  await page.goto('/')
  const chip = page.getByRole('button', { name: /active task/i })
  await expect(chip).toBeVisible()

  // Assert: the chip, cut short, shares its row with the theme's toggle, which
  // keeps its place to the chip's right.
  const drawn = await edgesOf(chip)
  const toggle = await edgesOf(page.getByRole('button', { name: /change theme/i }))
  expect(Math.abs(middleOf(toggle) - middleOf(drawn)), "the toggle on the chip's row").toBeLessThan(
    2,
  )
  expect(toggle.left, 'the toggle right of the chip').toBeGreaterThanOrEqual(drawn.right)
})

// markHue is the color the first mark inside an element is drawn in.
function markHue(locator: Locator): Promise<string> {
  return locator
    .locator('svg')
    .first()
    .evaluate((mark) => getComputedStyle(mark).color)
}

test("a started task's mark is drawn in one hue in the header and the Tasks list", async ({
  page,
}) => {
  // Arrange: the started task, streamed and listed.
  await streams(
    page,
    withTasks({ available: true, reason: '', active: started, linked: [started] }),
  )
  await page.route('**/api/tasks', (route) => route.fulfill({ json: listOf(started) }))
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/')
  await openSection(page, 'Tasks')

  // Act
  const inHeader = await markHue(page.getByRole('button', { name: /active task/i }))
  const inList = await markHue(page.getByRole('list', { name: 'Tasks' }).getByRole('button'))

  // Assert
  expect(inList).toBe(inHeader)
})

// markSeat says where a row's mark sits beside its description: centered on
// the first of the lines the description wraps onto — to within half a pixel,
// what rounding leaves — or how far down the block of them its middle is.
function markSeat(row: Element, description: string): string {
  const mark = row.querySelector('svg')
  const words = [...row.querySelectorAll('span')].find((span) => span.textContent === description)
  if (mark === null || words === undefined) {
    return 'not drawn'
  }

  const block = words.getBoundingClientRect()
  const line = parseFloat(getComputedStyle(words).lineHeight)
  const seat = mark.getBoundingClientRect()
  const middle = (seat.top + seat.bottom) / 2 - block.top
  if (Math.abs(middle - line / 2) <= 0.5) {
    return 'first line'
  }

  return `middle ${String(middle)} px down a ${String(block.height)} px block of ${String(line)} px lines`
}

// wordy is a task whose description wraps onto several lines wherever it is
// drawn.
const wordy = {
  ...tracking,
  description:
    `PROJ-1: ${'Refuse to start when the configuration names a forge it does not know. '.repeat(3)}`.trim(),
} satisfies Task

test("a mark sits centered on a wrapped description's first line, in the list and the card", async ({
  page,
}) => {
  // Arrange: the task, streamed and listed, and the issue's card open.
  await streams(page, withTasks({ available: true, reason: '', linked: [wordy] }))
  await page.route('**/api/tasks', (route) => route.fulfill({ json: listOf(wordy) }))
  await page.setViewportSize({ width: 1024, height })
  await page.goto('/')
  await page.getByRole('button', { name: /^PROJ-1/ }).click()
  const card = page.getByRole('region', { name: 'Tasks' }).getByRole('listitem')
  await expect(card).toBeVisible()

  // Act
  const onCard = await card.evaluate(markSeat, wordy.description)
  await openSection(page, 'Tasks')
  const row = page.getByRole('list', { name: 'Tasks' }).getByRole('button')
  const inList = await row.evaluate(markSeat, wordy.description)

  // Assert
  expect(onCard, 'on the card').toBe('first line')
  expect(inList, 'in the list').toBe('first line')
})

// A refusal in two lines, as a hook's output can make one.
const twoLines = 'Taskwarrior refused the command:\nthe on-add hook said no'

// The refusals that keep their lines: a track's, on the issue's card, and a
// failed read's, over the Tasks list.
const refusals = [
  {
    of: "a track's",
    act: async (page: Page) => {
      await page.getByRole('button', { name: /^PROJ-1/ }).click()
      await page.getByRole('button', { name: 'Track in Taskwarrior' }).click()
    },
  },
  {
    of: "a failed read's",
    act: async (page: Page) => {
      await page
        .getByRole('navigation', { name: 'Sections' })
        .getByRole('button', { name: 'Tasks', exact: true })
        .click()
    },
  },
]

for (const { of, act } of refusals) {
  test(`${of} refusal keeps its line breaks`, async ({ page }) => {
    // Arrange: a read and a track Taskwarrior refuses in two lines.
    await streams(page, withTasks({ available: true, reason: '', linked: [] }))
    await page.route('**/api/tasks', (route) =>
      route.fulfill(refused(502, 'unreachable', twoLines)),
    )
    await page.route('**/api/tasks/track', (route) =>
      route.fulfill(refused(422, 'unprocessable', twoLines)),
    )
    await page.goto('/')

    // Act
    await act(page)

    // Assert
    const said = page.getByRole('alert').filter({ hasText: 'on-add hook' })
    expect(await said.innerText()).toBe(twoLines)
  })
}

// renew is a task for no issue, with a priority and a tag, to narrow to.
const renew = {
  ...tracking,
  uuid: '6a2e8b4d-0c3f-4d9e-b7a1-4f2c3d5e6b7a',
  id: 2,
  description: 'Renew the staging certificate',
  priority: 'H',
  tags: ['ops'],
  urgency: 4.1,
  issue_key: '',
} satisfies Task

for (const theme of themes) {
  for (const width of widths) {
    test(`a narrowed, sorted task list fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange: two tasks, in this theme, at this width.
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await streams(page, withTasks({ available: true, reason: '', linked: [tracking] }))
      await page.route('**/api/tasks', (route) => route.fulfill({ json: listOf(tracking, renew) }))
      await page.goto('/')
      await openSection(page, 'Tasks')

      // Act: narrow to priority H, sort by tag, type a filter, and Tab once
      // round the page.
      const narrow = page.getByRole('group', { name: 'Filter' })
      await narrow.getByRole('button', { name: 'priority H 1' }).click()
      await page.getByRole('combobox', { name: 'Sort' }).selectOption('By tag')
      await page.getByRole('searchbox', { name: 'Search' }).fill('staging')
      await expect(page.getByText('1 of 2 tasks matches, by tag.')).toBeVisible()
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert: nothing scrolls sideways, nor the page down; Tab reaches the
      // sort, the filter and the pressed chip, each in view; and axe finds
      // nothing, a pressed chip's contrast included.
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
      expect(reached, 'reached by Tab').toEqual(
        expect.arrayContaining(['Sort', 'Search', 'priority H 1']),
      )
      expect(missed, 'never reached by Tab').toEqual([])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }
}
