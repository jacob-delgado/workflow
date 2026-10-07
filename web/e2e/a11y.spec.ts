import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Locator, type Page } from '@playwright/test'
import type { PullRequestDraft, Snapshot } from '../src/api/generated/types.gen.ts'
import {
  height,
  openFirstRun,
  openSection,
  pinTheme,
  sectionNames as populatedSectionNames,
  themes,
} from './cockpit.ts'
import { streams } from './tabwalk.ts'

// Every section, in both themes: a light theme is only real once its contrast
// holds up, so the scan runs the whole cockpit in each. The section labels are
// the nav buttons' accessible names and the content heading's text.
const sectionNames = [
  'Issues',
  'Branch',
  'Review',
  'Messaging',
  'Reviews',
  'Tasks',
  'Summary',
  'Repositories',
  'Settings',
]

// Scan the resting state, not mid-animation frames: reduced motion collapses
// transitions to instant, so axe never samples a half-faded element (whose
// transient blended colors are a false contrast failure).
test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
})

// settled is what shows once a section has drawn what it will with no API to
// answer it: Reviews, Tasks, Summary, Repositories and Settings each read their own endpoint
// after their heading appears, and the read fails here, so the scan waits on
// its Try again — the first, where Settings offers one for each of its reads;
// every other section settles with its heading.
function settled(page: Page, name: string): Locator {
  return ['Reviews', 'Tasks', 'Summary', 'Repositories', 'Settings'].includes(name)
    ? page.getByRole('button', { name: 'Try again' }).first()
    : page.getByRole('heading', { level: 1, name })
}

// unreachable is the answer a read gets when the service behind it is down.
const unreachable = {
  status: 502,
  contentType: 'application/problem+json',
  body: JSON.stringify({
    type: 'https://jacob-delgado.github.io/workflow/docs/errors/#unreachable',
    title: 'Upstream unreachable',
    status: 502,
    detail: 'the service could not be reached; check the network, then try again',
    code: 'unreachable',
  }),
}

// scan returns the WCAG A/AA violations axe finds on whatever is on screen.
async function scan(page: Page) {
  const { violations } = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()

  return violations
}

for (const theme of themes) {
  test(`no accessibility violations across the sections in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: pin the theme before the app paints, so the whole run is in it,
    // and answer the health read as a --dry-run server would, so the read-only
    // banner is on screen for every scan (the hermetic server has no API).
    await pinTheme(page, theme)
    await page.route('**/api/health', (route) =>
      route.fulfill({
        json: { version: '1.2.3', dry_run: true, forge_noun: 'pull request', forge_sigil: '#' },
      }),
    )
    // The review queue's read, the task list's and the configuration's fail,
    // so each reason and its Try again are scanned, whenever the answer comes; the
    // populated build scans the queue, the tasks and the form themselves.
    await page.route('**/api/reviews', (route) => route.fulfill(unreachable))
    await page.route('**/api/tasks', (route) => route.fulfill(unreachable))
    await page.route('**/api/repositories', (route) => route.fulfill(unreachable))
    await page.route('**/api/config', (route) => route.fulfill(unreachable))
    await page.goto('/')
    await expect(page.getByText(/every write is held back/i)).toBeVisible()

    const nav = page.getByRole('navigation', { name: 'Sections' })

    for (const name of sectionNames) {
      // Act: open the section and let it settle.
      await nav.getByRole('button', { name, exact: true }).click()
      await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
      await expect(settled(page, name)).toBeVisible()

      // Assert: axe finds nothing on this section in this theme.
      const violations = await scan(page)
      const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
      expect(violations, `${theme} / ${name}: ${summary}`).toEqual([])
    }
  })
}

for (const theme of themes) {
  test(`no accessibility violations in the first-run setup in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: a server with no configuration file, which Settings sets up.
    await openFirstRun(page, { width: 1024, height }, theme)

    // Act: a check that does not pass, which offers to write it anyway.
    await page.route('**/api/config/setup', (route) =>
      route.request().method() === 'POST'
        ? route.fulfill({
            status: 422,
            contentType: 'application/problem+json',
            body: JSON.stringify({
              type: 'https://jacob-delgado.github.io/workflow/docs/errors/#check-failed',
              title: 'Check failed',
              status: 422,
              detail: 'Jira did not accept the token; check it, or keep it anyway',
              code: 'check_failed',
            }),
          })
        : route.fallback(),
    )
    await page.getByRole('textbox', { name: 'Address' }).fill('https://jira.example.com')
    await page.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }).click()
    await expect(page.getByRole('button', { name: 'Write it anyway' })).toBeVisible()

    // Assert: axe finds nothing on the form or its refusal in this theme.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / first-run setup: ${summary}`).toEqual([])
  })
}

for (const theme of themes) {
  test(
    `no accessibility violations across the populated sections in the ${theme} theme`,
    {
      tag: '@populated',
    },
    async ({ page }) => {
      // Arrange: pin the theme before the app paints, and open the checked-out
      // issue, so its detail and work story are on screen beside the list.
      await pinTheme(page, theme)
      await page.goto('/')
      await page.getByRole('button', { name: /redact tokens before/i }).click()
      await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()

      for (const name of populatedSectionNames) {
        // Act: open the section and let it settle.
        await openSection(page, name)

        // Assert: axe finds nothing on this section, filled, in this theme.
        const violations = await scan(page)
        const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
        expect(violations, `${theme} / populated ${name}: ${summary}`).toEqual([])
      }
    },
  )
}

// The steps a click opens on the populated build before a write goes out:
// each is a group, named for what it asks.
const confirmSteps = [
  { step: 'push confirmation', section: 'Branch', opener: 'Push branch', group: /^Push / },
  {
    step: 'discard confirmation',
    section: 'Branch',
    opener: 'Discard internal/config/redact.go…',
    group: 'Discard the changes to internal/config/redact.go?',
  },
  {
    step: 'announcement preview',
    section: 'Slack',
    opener: 'Announce to Slack',
    group: 'Announcement preview',
  },
  {
    step: 'summary preview',
    section: 'Summary',
    opener: 'Post…',
    group: 'Summary preview',
  },
  {
    step: 'forget confirmation in People and groups',
    section: 'Settings',
    opener: 'Forget carla…',
    group: 'Forget carla?',
  },
  {
    step: 'remove confirmation in Local data',
    section: 'Settings',
    opener: 'Remove cache…',
    group: 'Remove workflow.db?',
  },
]

for (const theme of themes) {
  for (const { step, section, opener, group } of confirmSteps) {
    test(
      `no accessibility violations in the ${step} in the ${theme} theme`,
      { tag: '@populated' },
      async ({ page }) => {
        // Arrange: the populated cockpit in this theme, on the step's section.
        await pinTheme(page, theme)
        await page.goto('/')
        await openSection(page, section)

        // Act: open the step.
        await page.getByRole('button', { name: opener }).click()
        await expect(page.getByRole('group', { name: group })).toBeVisible()

        // Assert: axe finds nothing on the step and the section around it.
        const violations = await scan(page)
        const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
        expect(violations, `${theme} / populated ${step}: ${summary}`).toEqual([])
      },
    )
  }
}

// A snapshot with more issues than its page carries, so the list, its view
// select, filter, Where buttons and "Load more" are all on screen for the scan.
const issuesSnapshot = {
  issues: {
    total: 3,
    start_at: 0,
    unavailable: [],
    issues: [
      {
        key: 'PROJ-1',
        tracker: 'jira',
        summary: 'Redact tokens before they reach the request log',
        status: 'In Progress',
        status_category: 'indeterminate',
        type: 'Bug',
        priority: 'High',
      },
      {
        key: 'PROJ-2',
        tracker: 'jira',
        summary: 'Document the token flow',
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
  tasks: { available: true, reason: '', linked: [] },
  here: '/home/ana/src/api',
} satisfies Snapshot

const issueDetail = {
  ...issuesSnapshot.issues.issues[0],
  reporter: 'Ana Lopez',
  assignee: 'octocat',
  description: 'The request log records every header, so a bearer token lands in it.',
  comments: [{ author: 'Sam Ortiz', body: "Repro'd on main.", created: '2026-09-18T15:04:00Z' }],
  comment_total: 3,
  url: 'https://jira.example.com/browse/PROJ-1',
}

for (const theme of themes) {
  test(`no accessibility violations in the issue list and detail in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: the hermetic server has no API, so the stream, the views and the
    // issue are answered here — enough for the list's controls and the detail.
    await pinTheme(page, theme)
    await streams(page, issuesSnapshot)
    await page.route('**/api/views', (route) =>
      route.fulfill({
        json: {
          views: [
            { name: 'Assigned to me', jql: 'assignee = currentUser()' },
            { name: 'Team bugs', jql: 'type = Bug' },
          ],
        },
      }),
    )
    await page.route('**/api/issues/PROJ-1', (route) => route.fulfill({ json: issueDetail }))
    await page.goto('/')
    await expect(page.getByRole('button', { name: /load more/i })).toBeVisible()
    await expect(page.getByRole('combobox', { name: 'View' })).toBeVisible()

    // Act: narrow the list to a place, so a pressed Where button is scanned,
    // then open the first issue and let its detail land.
    const inProgress = page
      .getByRole('group', { name: 'Filter' })
      .getByRole('button', { name: /^In Progress/ })
    await inProgress.click()
    await expect(inProgress).toHaveAttribute('aria-pressed', 'true')
    await page.getByRole('button', { name: /redact tokens/i }).click()
    await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()

    // Assert: axe finds nothing on the list beside the open detail.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / issues: ${summary}`).toEqual([])
  })
}

const openedPull = {
  number: 7,
  url: 'https://forge.example.com/pull/7',
  title: 'fix: redact tokens before they reach the request log',
  state: 'open',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'unknown',
}

// The pull request the branch's work composes, as the draft read answers it.
const pullDraft = {
  title: openedPull.title,
  body: 'Redacts the Authorization header.',
  base: 'main',
  head: 'fix/PROJ-1',
  draft: false,
  needs_push: false,
  reviewers: ['ana', 'acme/control-plane'],
  templates: [],
  template: '',
} satisfies PullRequestDraft

for (const theme of themes) {
  test(`no accessibility violations in the pull request form in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: a stream with no pull request yet, and the draft answered here.
    await pinTheme(page, theme)
    await streams(page, issuesSnapshot)
    await page.route('**/api/pull-request/draft', (route) => route.fulfill({ json: pullDraft }))
    await page.goto('/')
    await openSection(page, 'Review')

    // Act: compose the pull request, which opens as a form to edit.
    await page.getByRole('button', { name: 'Open a pull request' }).click()
    await expect(page.getByRole('form', { name: 'Open a pull request' })).toBeVisible()

    // Assert: axe finds nothing on the form and its seven fields.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / pull request form: ${summary}`).toEqual([])
  })
}

for (const theme of themes) {
  test(`no accessibility violations in the offers after opening in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: a stream with no pull request yet, and the draft, the open and
    // the link answered here, so the open's outcome, its offers and a done
    // offer's status line are all on screen for the scan.
    await pinTheme(page, theme)
    await streams(page, issuesSnapshot)
    await page.route('**/api/pull-request/draft', (route) => route.fulfill({ json: pullDraft }))
    await page.route('**/api/pull-request', (route) =>
      route.fulfill({
        json: {
          pull: openedPull,
          follow_ups: [
            { action: 'link', issue_key: 'PROJ-1' },
            { action: 'transition', issue_key: 'PROJ-1', status: 'In Review' },
          ],
        },
      }),
    )
    await page.route('**/api/issues/PROJ-1/link', (route) => route.fulfill({ json: openedPull }))
    await page.goto('/')
    await page
      .getByRole('navigation', { name: 'Sections' })
      .getByRole('button', { name: 'Review', exact: true })
      .click()
    await page.getByRole('button', { name: 'Open a pull request' }).click()
    await page.getByRole('button', { name: 'Open pull request' }).click()

    // Act: link it, leaving the move offered beside what the link said.
    await page.getByRole('button', { name: 'Link it on PROJ-1' }).click()
    await expect(page.getByText('Linked #7 on PROJ-1.')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Move PROJ-1 to In Review' })).toBeVisible()

    // Assert: axe finds nothing on the outcome and its offers.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / offers: ${summary}`).toEqual([])
  })
}

// A working tree with a file wholly staged, one partly staged and one the
// index does not hold, on a branch with nothing to push, so each file's stage
// or unstage button, Stage all and the commit form are all on screen.
const workingTreeSnapshot = {
  ...issuesSnapshot,
  branch: {
    name: 'fix/PROJ-1',
    issue_link: '',
    detached: false,
    head: 'abc1234',
    upstream: 'origin/fix/PROJ-1',
    push_remote: 'origin',
    ahead: 0,
    behind: 0,
    base: 'origin/main',
    commits: [],
  },
  changes: {
    changes: [
      {
        path: 'internal/wiring/reqlog.go',
        kind: 'modified',
        staged: true,
        has_unstaged: false,
        conflicted: false,
      },
      {
        path: 'internal/config/redact.go',
        kind: 'modified',
        staged: true,
        has_unstaged: true,
        conflicted: false,
      },
      {
        path: 'notes.txt',
        kind: 'untracked',
        staged: false,
        has_unstaged: true,
        conflicted: false,
      },
    ],
  },
  here: '/home/ana/src/api',
} satisfies Snapshot

for (const theme of themes) {
  test(`no accessibility violations in the working tree in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: the stream's working tree, and the stage answered here, so a
    // file's outcome line is on screen beside the buttons and the form.
    await pinTheme(page, theme)
    await streams(page, workingTreeSnapshot)
    await page.route('**/api/stage', (route) =>
      route.fulfill({ json: workingTreeSnapshot.changes }),
    )
    await page.goto('/')
    await page
      .getByRole('navigation', { name: 'Sections' })
      .getByRole('button', { name: 'Branch' })
      .click()

    // Act: stage the untracked file.
    await page.getByRole('button', { name: 'Stage notes.txt' }).click()
    await expect(page.getByText('Staged notes.txt.')).toBeVisible()

    // Assert: axe finds nothing on the working tree and its commit form.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / working tree: ${summary}`).toEqual([])
  })
}

// refused is what a write gets when the server cannot complete it.
const refused = {
  status: 500,
  contentType: 'application/problem+json',
  body: JSON.stringify({
    type: 'https://jacob-delgado.github.io/workflow/docs/errors/#internal',
    title: 'Internal error',
    status: 500,
    detail:
      'the request could not be completed; try again, and run workflow doctor if it keeps failing',
    code: 'internal',
  }),
}

for (const theme of themes) {
  test(`no accessibility violations beside a refused write in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: the stream's working tree, and a stage the server refuses.
    await pinTheme(page, theme)
    await streams(page, workingTreeSnapshot)
    await page.route('**/api/stage', (route) => route.fulfill(refused))
    await page.goto('/')
    await openSection(page, 'Branch')

    // Act: stage the untracked file, and let the refusal land.
    await page.getByRole('button', { name: 'Stage notes.txt' }).click()
    await expect(page.getByRole('alert')).toHaveText(/could not be completed/)

    // Assert: axe finds nothing on the refusal and the working tree around it.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / refused write: ${summary}`).toEqual([])
  })
}
