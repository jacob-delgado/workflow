import { expect, type Locator, type Page } from '@playwright/test'
import { readings } from '../support/cockpit.ts'

// The loop's steps on the demo's page, written once for the screenshots,
// which take them briskly to reach the screen each shows, and for the
// walkthrough, which films them at a person's pace. Each step acts through
// its pace, then waits for what the page says it did before the next begins,
// so a slow machine slows a step down but never skips one.

// Pace is how a step acts on the page: how it clicks a control, how it types
// into a field, and the beat it holds once the page shows what the step did,
// captioned with what that is.
export interface Pace {
  click: (target: Locator) => Promise<void>
  type: (target: Locator, text: string) => Promise<void>
  beat: (caption: string) => Promise<void>
}

// brisk is the screenshots' pace: a click as Playwright makes it, a field
// filled at once, and no beat.
export const brisk: Pace = {
  click: (target) => target.click(),
  type: (target, text) => target.fill(text),
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

// goTo opens a section from the rail and waits for it to settle: its heading
// up and none of its own reads in flight.
export async function goTo(page: Page, pace: Pace, name: string): Promise<void> {
  await pace.click(
    page.getByRole('navigation', { name: 'Sections' }).getByRole('button', { name, exact: true }),
  )
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(readings(page)).toHaveCount(0)
}

// startWork starts work on the issue the detail shows, on a branch of its own.
export async function startWork(page: Page, pace: Pace): Promise<void> {
  await pace.click(page.getByRole('button', { name: 'Start work', exact: true }))
  await expect(said(page, `Started work on ${workedBranch}.`)).toBeVisible()
  await pace.beat(`Started work on ${workedBranch}`)
}

// stageAll stages every change in the working tree.
async function stageAll(page: Page, pace: Pace): Promise<void> {
  await pace.click(page.getByRole('button', { name: 'Stage all', exact: true }))
  await expect(said(page, 'Staged every change.')).toBeVisible()
  await pace.beat('Every change staged')
}

// stage stages the one file at path, done once its row offers to unstage it.
export async function stage(page: Page, pace: Pace, path: string): Promise<void> {
  await pace.click(page.getByRole('button', { name: `Stage ${path}`, exact: true }))
  await expect(said(page, `Staged ${path}.`)).toBeVisible()
  await expect(page.getByRole('button', { name: `Unstage ${path}`, exact: true })).toBeVisible()
  await pace.beat(`${path} staged`)
}

// commit commits what is staged as a fix, in the scope the form proposes,
// with subject, done once the branch lists it: the form says so a frame of
// the stream before the branch shows it.
export async function commit(page: Page, pace: Pace, subject: string): Promise<void> {
  const form = page.getByRole('form', { name: 'Commit staged changes' })
  await form.getByLabel('Type').selectOption('fix')
  await pace.type(form.getByLabel('Subject'), subject)
  await pace.click(form.getByRole('button', { name: 'Commit staged changes' }))
  await expect(form.getByRole('status')).toHaveText(
    new RegExp(`^Committed [0-9a-f]{7} fix\\(config\\): ${subject}\\.$`),
  )
  await expect(
    page
      .getByRole('region', { name: 'Commits', exact: true })
      .getByRole('listitem')
      .filter({ hasText: `fix(config): ${subject}` }),
  ).toBeVisible()
  await pace.beat(`Committed fix(config): ${subject}`)
}

// push pushes the branch, confirming the push it asks about.
export async function push(page: Page, pace: Pace): Promise<void> {
  await pace.click(page.getByRole('button', { name: 'Push branch' }))
  await pace.click(
    page.getByRole('group', { name: /^Push / }).getByRole('button', { name: 'Push', exact: true }),
  )
  await expect(said(page, `Pushed ${workedBranch}.`)).toBeVisible()
  await pace.beat(`Pushed ${workedBranch}`)
}

// openPull opens the branch's pull request from the Review section, with the
// title, base, reviewers and body the form proposes.
async function openPull(page: Page, pace: Pace): Promise<void> {
  await pace.click(page.getByRole('button', { name: 'Open a pull request' }))
  const form = page.getByRole('form', { name: 'Open a pull request' })
  await expect(form.getByRole('textbox', { name: 'Reviewers' })).toHaveValue(/sam\.ortiz/)
  await pace.beat('The pull request composed, its reviewers proposed')
  await pace.click(form.getByRole('button', { name: 'Open pull request', exact: true }))
  await expect(said(page, 'Opened pull request #128.')).toBeVisible()
  await pace.beat('#128 opened, its announcement waiting for CI')
}

// ciChecks is the Review section's CI checks, named for how many are done.
function ciChecks(page: Page, done: number): Locator {
  return page.getByRole('region', { name: `CI checks ${String(done)} of 4 done` })
}

// ciReads waits for the Review section's CI to read done of its four checks
// done.
export async function ciReads(page: Page, pace: Pace, done: number): Promise<void> {
  await expect(ciChecks(page, done)).toBeVisible({ timeout: ciTakes })
  await pace.beat(`CI ${String(done)} of 4 done`)
}

// toOpenPull takes the loop from the cockpit, PROJ-412 picked, to #128 open
// on the Review section: start work, stage every change, commit, push, open.
export async function toOpenPull(page: Page, pace: Pace): Promise<void> {
  await startWork(page, pace)
  await goTo(page, pace, 'Branch')
  await stageAll(page, pace)
  await commit(page, pace, 'redact tokens in the request log')
  await push(page, pace)
  await goTo(page, pace, 'Review')
  await openPull(page, pace)
}
