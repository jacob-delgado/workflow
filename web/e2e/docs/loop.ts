import { expect, type Locator, type Page } from '@playwright/test'
import { readings, workedIssue } from '../support/cockpit.ts'

// The loop's steps on the demo's page, written once for the screenshots,
// which take them briskly to reach the screen each shows, and for the
// walkthrough, which films them at a person's pace. Each step acts through
// its pace, then waits for what the page says it did before the next begins,
// so a slow machine slows a step down but never skips one.

// Pace is how a step acts on the page: how it clicks a control, how it types
// into a field, how it chooses an option of a select by its value or label,
// and the beat it holds once the page shows what the step did, captioned
// with what that is, on the element that shows it.
export interface Pace {
  click: (target: Locator) => Promise<void>
  type: (target: Locator, text: string) => Promise<void>
  choose: (select: Locator, value: string) => Promise<void>
  beat: (caption: string, shown: Locator) => Promise<void>
}

// brisk is the screenshots' pace: a click as Playwright makes it, a field
// filled and an option chosen at once, and no beat.
export const brisk: Pace = {
  click: (target) => target.click(),
  type: (target, text) => target.fill(text),
  choose: async (select, value) => {
    await select.selectOption(value)
  },
  beat: () => Promise.resolve(),
}

// workedBranch is the branch starting work on PROJ-412 makes.
const workedBranch = 'fix/PROJ-412-redact-tokens-in-the-request-log'

// ciTakes is how long the demo's CI may take to read as it stands, once its
// checks have had time to: four seconds of checks, the server's two-second
// read of CI, a frame of the stream and the demo's read latency on each.
const ciTakes = 20_000

// said is a status line the page shows, whose text is what a write did.
function said(page: Page, text: string | RegExp): Locator {
  return page.getByRole('status').filter({ hasText: text })
}

// workStory is the selected issue's work story, in its detail.
export function workStory(page: Page): Locator {
  return page.getByRole('article').getByRole('region', { name: 'Work story' })
}

// goTo opens a section from the rail and waits for it to settle: its heading
// up and none of its own reads in flight.
export async function goTo(page: Page, pace: Pace, name: string): Promise<void> {
  await pace.click(
    page.getByRole('navigation', { name: 'Sections' }).getByRole('button', { name, exact: true }),
  )
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(readings(page)).toHaveCount(0)
}

// startButton is the selected issue's Start work, on a branch of its own.
function startButton(page: Page): Locator {
  return page.getByRole('button', { name: 'Start work', exact: true })
}

// pickIssue selects PROJ-412 in the Issues list, its detail beside it.
export async function pickIssue(page: Page, pace: Pace): Promise<void> {
  await pace.click(workedIssue(page))
  await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()
  const start = startButton(page)
  await expect(start).toBeVisible()
  await pace.beat('PROJ-412 picked, its work not started', start)
}

// startWork starts work on the issue the detail shows, on a branch of its
// own, done once its work story's Branch stage names the branch, which the
// status says only once the story shows it.
export async function startWork(page: Page, pace: Pace): Promise<void> {
  await pace.click(startButton(page))
  await expect(said(page, `Started work on ${workedBranch}.`)).toBeVisible()
  const stage = workStory(page).getByRole('listitem').filter({ hasText: workedBranch })
  await expect(stage).toBeVisible()
  await pace.beat(`Started work on ${workedBranch}`, stage)
}

// moveIntoProgress moves PROJ-412 to In Progress through its detail's Change
// status, since starting work leaves the issue where it stood, done once the
// detail reads it In Progress.
export async function moveIntoProgress(page: Page, pace: Pace): Promise<void> {
  const detail = page.getByRole('article')
  await pace.click(detail.getByRole('button', { name: 'Change status', exact: true }))
  const form = page.getByRole('form', { name: 'Change the status of PROJ-412' })
  await pace.choose(form.getByLabel('New status'), 'In Progress (Start Progress)')
  await pace.click(form.getByRole('button', { name: 'Change to In Progress', exact: true }))
  const moved = said(page, 'Changed PROJ-412 to In Progress.')
  await expect(moved).toBeVisible()
  await expect(detail.getByText('In Progress', { exact: true }).first()).toBeVisible()
  await pace.beat('PROJ-412 moved to In Progress', moved)
}

// stageAll stages every change in the working tree, done once no file in it
// still offers to be staged. Its beat holds the notice under the files, and
// the files above it, each marked staged, are held whole in that frame too,
// since they are what its caption says; not the whole Working tree, which
// holds the commit form below and is taller than the window.
async function stageAll(page: Page, pace: Pace): Promise<void> {
  const tree = page.getByRole('region', { name: 'Working tree' })
  await pace.click(tree.getByRole('button', { name: 'Stage all', exact: true }))
  const staged = said(page, 'Staged every change.')
  await expect(staged).toBeVisible()
  await expect(tree.getByRole('button', { name: /^Stage (?!all$)/ })).toHaveCount(0)
  await pace.beat('Every change staged', staged)
  await expect(tree.getByRole('list').first(), 'the files staged').toBeInViewport({ ratio: 1 })
}

// stage stages the one file at path, done once its row offers to unstage it.
export async function stage(page: Page, pace: Pace, path: string): Promise<void> {
  await pace.click(page.getByRole('button', { name: `Stage ${path}`, exact: true }))
  await expect(said(page, `Staged ${path}.`)).toBeVisible()
  const unstage = page.getByRole('button', { name: `Unstage ${path}`, exact: true })
  await expect(unstage).toBeVisible()
  await pace.beat(`${path} staged`, unstage)
}

// commit commits what is staged as a fix, in the scope the form proposes,
// with subject, done once the branch lists it, which the form says only once
// the branch shows it.
export async function commit(page: Page, pace: Pace, subject: string): Promise<void> {
  const form = page.getByRole('form', { name: 'Commit staged changes' })
  await pace.choose(form.getByLabel('Type'), 'fix')
  await pace.type(form.getByLabel('Subject'), subject)
  await pace.click(form.getByRole('button', { name: 'Commit staged changes' }))
  await expect(form.getByRole('status')).toHaveText(
    new RegExp(`^Committed [0-9a-f]{7} fix\\(config\\): ${subject}\\.$`),
  )
  const listed = page
    .getByRole('region', { name: 'Commits', exact: true })
    .getByRole('listitem')
    .filter({ hasText: `fix(config): ${subject}` })
  await expect(listed).toBeVisible()
  await pace.beat(`Committed fix(config): ${subject}`, listed)
}

// push pushes the branch, confirming the push it asks about, done once its
// tracking reads it pushed and nothing ahead of its upstream.
async function push(page: Page, pace: Pace): Promise<void> {
  await pace.click(page.getByRole('button', { name: 'Push branch' }))
  await pace.click(
    page.getByRole('group', { name: /^Push / }).getByRole('button', { name: 'Push', exact: true }),
  )
  const pushed = said(page, `Pushed ${workedBranch}.`)
  await expect(pushed).toBeVisible()
  await expect(page.getByRole('definition').filter({ hasText: /^0 ahead, / })).toBeVisible()
  await expect(page.getByText('not pushed yet', { exact: true })).toHaveCount(0)
  await pace.beat(`Pushed ${workedBranch}`, pushed)
}

// openPull opens the branch's pull request from the Review section, with the
// title, base, reviewers and body the form proposes, held composed once its
// title, base and reviewers are filled in.
async function openPull(page: Page, pace: Pace): Promise<void> {
  await pace.click(page.getByRole('button', { name: 'Open a pull request' }))
  const form = page.getByRole('form', { name: 'Open a pull request' })
  const reviewers = form.getByRole('textbox', { name: 'Reviewers' })
  await expect(form.getByRole('textbox', { name: 'Title', exact: true })).toHaveValue(/\S/)
  await expect(form.getByRole('textbox', { name: 'Base branch', exact: true })).toHaveValue(/\S/)
  await expect(reviewers).toHaveValue(/sam\.ortiz/)
  await pace.beat('The pull request composed, its reviewers proposed', reviewers)
  await pace.click(form.getByRole('button', { name: 'Open pull request', exact: true }))
  const opened = said(page, 'Opened pull request #128.')
  await expect(opened).toBeVisible()
  await pace.beat('#128 opened, its announcement waiting for CI', opened)
}

// ciChecks is the Review section's CI checks, named for how many are done.
function ciChecks(page: Page, done: number): Locator {
  return page.getByRole('region', { name: `CI checks ${String(done)} of 4 done` })
}

// ciReads waits for the Review section's CI to read done of its four checks
// done.
export async function ciReads(page: Page, pace: Pace, done: number): Promise<void> {
  const checks = ciChecks(page, done)
  await expect(checks).toBeVisible({ timeout: ciTakes })
  await pace.beat(`CI ${String(done)} of 4 done`, checks)
}

// toOpenPull takes the loop from the cockpit, PROJ-412 picked, to #128 open
// on the Review section: start work, move the issue into progress, stage
// every change, commit, push, open.
export async function toOpenPull(page: Page, pace: Pace): Promise<void> {
  await startWork(page, pace)
  await moveIntoProgress(page, pace)
  await goTo(page, pace, 'Branch')
  await stageAll(page, pace)
  await commit(page, pace, 'redact tokens in the request log')
  await push(page, pace)
  await goTo(page, pace, 'Review')
  await openPull(page, pace)
}

// armedForCI waits for the Slack section to say the announcement was armed
// as #128 opened, and waits for its CI to pass, never that a pull request
// is still to be opened.
export async function armedForCI(page: Page, pace: Pace): Promise<void> {
  const armed = page.getByText('Armed when #128 opened.', { exact: false })
  await expect(armed).toBeVisible()
  await expect(page.getByText(/^Open a pull request first/)).toHaveCount(0)
  await pace.beat('The announcement armed when #128 opened, waiting for CI', armed)
}

// announcedOnGreen waits for the Slack section to say the announcement went
// out itself once CI passed.
export async function announcedOnGreen(page: Page, pace: Pace): Promise<void> {
  const announced = page.getByText('Announced to #dev-workflow once CI passed.')
  await expect(announced).toBeVisible({ timeout: ciTakes })
  await pace.beat('Announced to #dev-workflow once CI passed', announced)
}

// storyDone waits for PROJ-412's work story to read announced, its last stage
// done, and holds the whole story, every stage of it in view.
export async function storyDone(page: Page, pace: Pace): Promise<void> {
  const story = workStory(page)
  const told = story.getByRole('listitem').filter({ hasText: /Announced/ })
  await expect(told).toBeVisible()
  await pace.beat("PROJ-412's work story, every stage done", story)
}
