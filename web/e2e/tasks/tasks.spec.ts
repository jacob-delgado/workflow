import type { Locator, Page } from '@playwright/test'
import type {
  Snapshot,
  Task,
  TaskFacet,
  TaskList,
  TasksSummary,
} from '../../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { expect, issuesOf, problem, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean } from '../support/reachable.ts'

// Your Taskwarrior tasks as the browser draws them, against answers given
// here: where a refusal sits against its row's buttons, how the header's task
// chip gives way in a narrow window, where a mark sits beside a description
// that wraps, the hue a task's mark is drawn in, and a refusal's line breaks.

// The values the tasks below hold, each as the server labels it.
const pending: TaskFacet = { kind: 'state', value: 'pending', label: 'pending' }
const priorityH: TaskFacet = { kind: 'priority', value: 'H', label: 'priority H' }
const noPriority: TaskFacet = { kind: 'priority', value: '', label: 'no priority' }
const noProject: TaskFacet = { kind: 'project', value: '', label: 'no project' }
const withIssue: TaskFacet = { kind: 'issue', value: 'linked', label: 'with issue' }
const noIssue: TaskFacet = { kind: 'issue', value: 'unlinked', label: 'no issue' }
const tagOps: TaskFacet = { kind: 'tag', value: 'ops', label: '+ops' }
const noTag: TaskFacet = { kind: 'tag', value: '', label: 'no tag' }

// firstInEveryOrder is the ranks of a task the case does not sort.
const firstInEveryOrder = { urgency: 0, state: 0, id: 0, tag: 0, issue: 0, priority: 0 }

// tracking is a task Taskwarrior holds for PROJ-1, pending and not started,
// as the server answers it.
const tracking: Task = {
  uuid: '5f1d7a3c-9b2e-4c8d-a6f0-3e1b2c4d5a6f',
  id: 1,
  description: 'PROJ-1: Refuse to start when the config names an unknown forge',
  status: 'pending',
  state: 'pending',
  project: '',
  priority: '',
  tags: [],
  entry: '2026-09-20T10:00:00Z',
  modified: '2026-09-20T10:00:00Z',
  urgency: 8.2,
  annotations: [],
  issue_key: 'PROJ-1',
  issue_url: '',
  facets: [pending, noPriority, noProject, withIssue, noTag],
  ranks: firstInEveryOrder,
  searchable: [
    'proj-1: refuse to start when the config names an unknown forge',
    '',
    'proj-1',
    '#1',
  ],
}

// started is the same task, started.
const started: Task = {
  ...tracking,
  start: '2026-09-28T09:00:00Z',
  state: 'started',
  facets: [
    { kind: 'state', value: 'started', label: 'started' },
    noPriority,
    noProject,
    withIssue,
    noTag,
  ],
}

// withTasks is a stream frame of one issue, PROJ-1, and of your tasks as given.
function withTasks(tasks: TasksSummary): Snapshot {
  return snapshotWith({
    issues: issuesOf([
      {
        key: 'PROJ-1',
        tracker: 'jira',
        summary: 'Refuse an unknown forge',
        status: 'To Do',
        status_category: 'new',
        type: 'Task',
      },
    ]),
    tasks,
  })
}

// listOf is the task list as the server answers a read of it: the tasks, and
// the values its filter offers, in the server's order.
function listOf(tasks: Task[], offered: TaskFacet[] = []): TaskList {
  return {
    available: true,
    reason: '',
    context: '',
    sync_available: false,
    said: '',
    tasks,
    facet_order: offered,
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
  // Arrange
  await streams(page, withTasks({ available: true, reason: '', linked: [tracking] }))
  await page.route('**/api/tasks', (route) => route.fulfill({ json: listOf([tracking]) }))
  await page.route('**/api/tasks/*/start', (route) =>
    route.fulfill(problem('conflict', 'the task is already in that state')),
  )
  await page.setViewportSize({ width: 1024, height })
  await page.goto('/')
  await openSection(page, 'Tasks')

  // Act
  await page.getByRole('button', { name: 'Start', exact: true }).click()
  await expect(page.getByRole('alert')).toBeVisible()

  // Assert
  const start = await edgesOf(page.getByRole('button', { name: 'Start', exact: true }))
  const done = await edgesOf(page.getByRole('button', { name: 'Mark done…', exact: true }))
  const refusal = await edgesOf(page.getByRole('alert'))
  expect(done.top, 'Mark done beside Start').toBe(start.top)
  expect(refusal.top, 'the refusal below both').toBeGreaterThanOrEqual(done.bottom)
})

test('the header keeps one row beside a long started task, at 640 px', async ({ page }) => {
  // Arrange
  // A started task whose description is longer than the header has room for.
  await streams(
    page,
    withTasks({ available: true, reason: '', active: started, linked: [started] }),
  )
  await page.setViewportSize({ width: 640, height })

  // Act
  await page.goto('/')
  const chip = page.getByRole('button', { name: /active task/i })
  await expect(chip).toBeVisible()

  // Assert
  const drawn = await edgesOf(chip)
  const toggle = await edgesOf(page.getByRole('button', { name: /change theme/i }))
  expect(Math.abs(middleOf(toggle) - middleOf(drawn)), "the toggle on the chip's row").toBeLessThan(
    2,
  )
  expect(toggle.left, 'the toggle right of the chip').toBeGreaterThanOrEqual(drawn.right)
})

// markHue is the color the first mark inside an element is drawn in: an icon
// hidden from assistive technology, which no role or name reaches, so it is
// found from the element it marks, as markSeat finds it.
function markHue(locator: Locator): Promise<string> {
  return locator.evaluate((element) => {
    const mark = element.querySelector('svg')

    return mark === null ? 'not drawn' : getComputedStyle(mark).color
  })
}

test("a started task's mark is drawn in one hue in the header and the Tasks list", async ({
  page,
}) => {
  // Arrange
  await streams(
    page,
    withTasks({ available: true, reason: '', active: started, linked: [started] }),
  )
  await page.route('**/api/tasks', (route) => route.fulfill({ json: listOf([started]) }))
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/')
  await openSection(page, 'Tasks')

  // Act
  const inHeader = await markHue(page.getByRole('button', { name: /active task/i }))
  const inList = await markHue(page.getByRole('list', { name: 'Tasks' }).getByRole('button'))

  // Assert
  expect(inHeader, 'the header draws its mark').toMatch(/^rgb/)
  expect(inList, "the list's mark in the header's hue").toBe(inHeader)
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
const wordyDescription =
  `PROJ-1: ${'Refuse to start when the configuration names a forge it does not know. '.repeat(3)}`.trim()
const wordy: Task = {
  ...tracking,
  description: wordyDescription,
  searchable: [
    `proj-1: ${'refuse to start when the configuration names a forge it does not know. '.repeat(3)}`.trim(),
    '',
    'proj-1',
    '#1',
  ],
}

test("a mark sits centered on a wrapped description's first line, in the list and the card", async ({
  page,
}) => {
  // Arrange
  await streams(page, withTasks({ available: true, reason: '', linked: [wordy] }))
  await page.route('**/api/tasks', (route) => route.fulfill({ json: listOf([wordy]) }))
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

// twoLinesDrawn is the refusal as the page must draw it, a line break and all.
const twoLinesDrawn = /^Taskwarrior refused the command:\nthe on-add hook said no$/

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
      await openSection(page, 'Tasks')
    },
  },
]

for (const { of, act } of refusals) {
  test(`${of} refusal keeps its line breaks`, async ({ page }) => {
    // Arrange
    await streams(page, withTasks({ available: true, reason: '', linked: [] }))
    await page.route('**/api/tasks', (route) => route.fulfill(problem('unreachable', twoLines)))
    await page.route('**/api/tasks/track', (route) =>
      route.fulfill(problem('unprocessable', twoLines)),
    )
    await page.goto('/')

    // Act
    await act(page)

    // Assert
    // A pattern, since a string match folds whitespace and would pass with the
    // lines run together.
    await expect(page.getByRole('alert').filter({ hasText: 'on-add hook' })).toHaveText(
      twoLinesDrawn,
      { useInnerText: true },
    )
  })
}

// The three tasks below are listed together, each with its place in every
// order among them as the server's RanksOf gives it: every tie falls to the
// more urgent.

// renew is a task for no issue, with a priority and a tag, to narrow to.
const renew: Task = {
  ...tracking,
  uuid: '6a2e8b4d-0c3f-4d9e-b7a1-4f2c3d5e6b7a',
  id: 3,
  description: 'Renew the staging certificate',
  priority: 'H',
  tags: ['ops'],
  urgency: 4.1,
  issue_key: '',
  facets: [pending, priorityH, noProject, noIssue, tagOps],
  ranks: { urgency: 2, state: 2, id: 2, tag: 0, issue: 2, priority: 1 },
  searchable: ['renew the staging certificate', '', '', '+ops', '#3'],
}

// rotate is another task for no issue, with renew's priority but no tag: more
// urgent than renew, so it is listed after renew only by tag.
const rotate: Task = {
  ...tracking,
  uuid: '7b3f9c5e-1d4a-4e0f-a8b2-5a3d4e6f7c8b',
  id: 2,
  description: 'Rotate the staging database password',
  priority: 'H',
  urgency: 6.3,
  issue_key: '',
  facets: [pending, priorityH, noProject, noIssue, noTag],
  ranks: { urgency: 1, state: 1, id: 1, tag: 2, issue: 1, priority: 0 },
  searchable: ['rotate the staging database password', '', '', '#2'],
}

// trackingAmongThree is tracking listed with renew and rotate: the most urgent,
// with the lowest id and the only issue, but after renew by tag and last by
// priority, having neither.
const trackingAmongThree: Task = {
  ...tracking,
  ranks: { urgency: 0, state: 0, id: 0, tag: 1, issue: 0, priority: 2 },
}

// heldValues is the values the three tasks hold, in the order the server
// offers them.
const heldValues = [pending, priorityH, noPriority, noProject, tagOps, noTag, withIssue, noIssue]

for (const theme of themes) {
  for (const width of widths) {
    test(`a narrowed, sorted task list fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await streams(page, withTasks({ available: true, reason: '', linked: [tracking] }))
      // Handed over out of tag order, so only the ranks list renew first by tag.
      await page.route('**/api/tasks', (route) =>
        route.fulfill({ json: listOf([trackingAmongThree, rotate, renew], heldValues) }),
      )
      await page.goto('/')
      await openSection(page, 'Tasks')

      // Act
      const narrow = page.getByRole('group', { name: 'Filter' })
      await narrow.getByRole('button', { name: 'priority H 2' }).click()
      await page.getByRole('combobox', { name: 'Sort' }).selectOption('By tag')
      await page.getByRole('searchbox', { name: 'Search' }).fill('staging')
      await expect(page.getByText('2 of 3 tasks match, by tag.')).toBeVisible()

      // Assert
      await expect(page.getByRole('list', { name: 'Tasks' }).getByRole('button')).toHaveText([
        /Renew the staging certificate/,
        /Rotate the staging database password/,
      ])
      // The chip is pressed, so the scan checks a pressed chip's contrast.
      await expectReachableAndClean(page, { reaches: ['Sort', 'Search', 'priority H 2'] })
    })
  }
}
