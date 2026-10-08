import { expect, test, type Page } from '@playwright/test'
import type { IssueDetail, Snapshot, StatusChange } from '../../src/api/generated/types.gen.ts'
import { height, pinTheme, themes, widths } from '../support/cockpit.ts'
import { axeViolations, sidewaysScrollers, streams, walkTabOrder } from '../support/tabwalk.ts'

// Changing a Jira issue from its detail: its status with the fields the change
// needs, its assignee, and the work logged on it.

const issue = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Redact tokens before they reach the request log',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  priority: 'High',
} as const

const snapshot = {
  issues: { total: 1, start_at: 0, unavailable: [], issues: [issue] },
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

// statusChanges are what PROJ-1 offers: one that needs nothing, and one to
// Resolved whose form needs a date, a list and a choice.
const statusChanges: StatusChange[] = [
  {
    id: '11',
    name: 'Block',
    to_status: 'Blocked',
    to_status_category: 'indeterminate',
    fields: [],
  },
  {
    id: '21',
    name: 'Resolve Issue',
    to_status: 'Resolved',
    to_status_category: 'done',
    fields: [
      { id: 'duedate', name: 'Due date', kind: 'date', options: [] },
      {
        id: 'fixVersions',
        name: 'Fix versions',
        kind: 'option_list',
        options: [
          { id: '10', name: '1.4.0' },
          { id: '11', name: '1.5.0' },
        ],
      },
      {
        id: 'resolution',
        name: 'Resolution',
        kind: 'option',
        options: [
          { id: '1', name: 'Fixed' },
          { id: '2', name: "Won't Fix" },
        ],
      },
    ],
  },
]

const detail: IssueDetail = {
  ...issue,
  reporter: 'Ana Lopez',
  assignee: 'octocat',
  description: 'The request log records every header.',
  comments: [],
  comment_total: 0,
  url: 'https://jira.example.com/browse/PROJ-1',
}

// opensIssue serves PROJ-1 and the status changes it offers, keeps the body
// of each write sent to it, and opens its detail. It answers what was sent.
async function opensIssue(page: Page): Promise<unknown[]> {
  const sent: unknown[] = []
  await streams(page, snapshot)
  await page.route('**/api/issues/PROJ-1', (route) => route.fulfill({ json: detail }))
  await page.route('**/api/issues/PROJ-1/transitions', (route) => {
    if (route.request().method() === 'GET') {
      return route.fulfill({ json: statusChanges })
    }

    sent.push(route.request().postDataJSON())

    return route.fulfill({ json: { key: 'PROJ-1', status: 'Resolved' } })
  })
  await page.goto('/')
  await page.getByRole('button', { name: /redact tokens/i }).click()
  await expect(page.getByRole('button', { name: 'Change status' })).toBeVisible()

  return sent
}

// choosesResolved opens Change status and chooses the change to Resolved,
// whose form needs a date, a list and a choice.
async function choosesResolved(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Change status' }).click()
  await page.getByRole('combobox', { name: 'New status' }).selectOption('21')
}

test('a status change is sent with the fields its form filled', async ({ page }) => {
  // Arrange
  const sent = await opensIssue(page)
  await choosesResolved(page)
  const form = page.getByRole('form', { name: 'Change the status of PROJ-1' })
  await form.getByRole('textbox', { name: 'Due date' }).fill('2026-10-09')
  await form.getByRole('checkbox', { name: '1.4.0' }).check()
  await form.getByRole('combobox', { name: 'Resolution' }).selectOption({ label: 'Fixed' })

  // Act
  await page.getByRole('button', { name: 'Change to Resolved' }).click()

  // Assert
  await expect(
    page.getByRole('status').filter({ hasText: 'Changed PROJ-1 to Resolved.' }),
  ).toBeVisible()
  expect(sent).toEqual([
    {
      transition_id: '21',
      fields: [
        { id: 'duedate', text: '2026-10-09' },
        { id: 'fixVersions', option_ids: ['10'] },
        { id: 'resolution', option_id: '1' },
      ],
    },
  ])
})

for (const theme of themes) {
  for (const width of widths) {
    test(`the status change form fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange: the change to Resolved chosen, in this theme, at this width.
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await opensIssue(page)
      await choosesResolved(page)

      // Act: Tab once round the page.
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert: nothing scrolls sideways; Tab reaches the fields and the
      // buttons, each in view; and axe finds nothing.
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(reached, 'reached by Tab').toEqual(
        expect.arrayContaining([
          'New status',
          'Due date',
          'Resolution',
          'Cancel',
          'Change to Resolved',
        ]),
      )
      expect(missed, 'never reached by Tab').toEqual([])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }

  for (const action of ['Assign', 'Log work']) {
    test(`the ${action} form is reachable and clean in the ${theme} theme`, async ({ page }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width: widths[0], height })
      await opensIssue(page)
      await page.getByRole('button', { name: action }).click()

      // Act: Tab once round the page.
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(['Cancel', action]))
      expect(missed, 'never reached by Tab').toEqual([])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }
}
