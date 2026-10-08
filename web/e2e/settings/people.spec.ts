import { expect, test, type Page } from '@playwright/test'
import type { OwnerTag, PersonLink, SlackTarget } from '../../src/api/generated/types.gen.ts'
import { height, openCockpit, openSection } from '../support/cockpit.ts'

// Settings' People and groups area: whom each code owner is on Slack, chosen
// and saved at once, forgotten behind a confirm step, and the repository's
// user groups saved together. The flows run on the hermetic build, whose
// answers this spec gives and records; the layout runs in layout.spec.ts and
// a11y.spec.ts, on the populated build.

const ben = { id: 'U0BEN', label: 'Ben Ito' }
const carla = { id: 'U0CARLA', label: 'Carla Diaz' }
const pod = { id: 'S0POD', label: 'control-plane-pod' }
const reviewers = { id: 'S0API', label: 'api-reviewers' }

// asked is each write the page sent, as method, query and body.
type Asked = string[]

// answersPeople answers People and groups from a kept store the writes
// change, and records each write.
async function answersPeople(page: Page): Promise<Asked> {
  const asked: Asked = []
  let owners: OwnerTag[] = [
    { owner: 'carla', kind: 'user', state: 'linked', slack: carla },
    { owner: 'dan', kind: 'user', state: 'not_on_slack' },
    { owner: 'ben', kind: 'user', state: 'unlinked' },
  ]
  let groups: SlackTarget[] = [reviewers]
  await page.route('**/api/people**', (route) => {
    const request = route.request()
    const url = new URL(request.url())
    if (request.method() === 'PUT') {
      const link = request.postDataJSON() as PersonLink
      asked.push(`PUT ${JSON.stringify(link)}`)
      owners = owners.map((owner) =>
        owner.owner === link.owner ? { ...owner, state: 'linked', slack: ben } : owner,
      )
    }

    if (request.method() === 'DELETE') {
      asked.push(`DELETE ${url.search}`)
      owners = owners.filter((owner) => owner.owner !== url.searchParams.get('owner'))
    }

    return route.fulfill({ json: { owners } })
  })
  await page.route('**/api/repo-groups', (route) => {
    const request = route.request()
    if (request.method() === 'PUT') {
      const { ids } = request.postDataJSON() as { ids: string[] }
      asked.push(`PUT ${JSON.stringify(ids)}`)
      groups = [pod, reviewers].filter((group) => ids.includes(group.id))
    }

    return route.fulfill({ json: { repository: 'acme/widgets', groups } })
  })
  await page.route('**/api/slack/members**', (route) =>
    route.fulfill({ json: { entries: [ben, carla] } }),
  )
  await page.route('**/api/slack/groups', (route) =>
    route.fulfill({ json: { entries: [pod, reviewers] } }),
  )

  return asked
}

// opensPeople opens Settings and waits for its People and groups table.
async function opensPeople(page: Page): Promise<void> {
  await page.goto('/')
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Settings', exact: true })
    .click()
  await expect(page.getByRole('table', { name: 'Code owners on Slack' })).toBeVisible()
}

test('links an owner to a Slack member, saved at once', async ({ page }) => {
  // Arrange
  const asked = await answersPeople(page)
  await opensPeople(page)
  const choice = page.getByRole('combobox', { name: 'Slack for ben' })
  await expect(choice.getByRole('option', { name: 'Ben Ito' })).toBeAttached()

  // Act
  await choice.selectOption({ label: 'Ben Ito' })

  // Assert
  await expect(page.getByText('Saved: ben is Ben Ito.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Forget ben…' })).toBeVisible()
  expect(asked).toEqual(['PUT {"owner":"ben","slack_id":"U0BEN","not_on_slack":false}'])
})

test('forgets an owner once confirmed', async ({ page }) => {
  // Arrange
  const asked = await answersPeople(page)
  await opensPeople(page)
  await page.getByRole('button', { name: 'Forget dan…' }).click()
  const question = page.getByRole('group', { name: 'Forget dan?' })
  await expect(question).toBeFocused()

  // Act
  await question.getByRole('button', { name: 'Forget', exact: true }).click()

  // Assert
  await expect(page.getByText('Forgot dan: they are asked about again.')).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Slack for dan' })).toBeHidden()
  expect(asked).toEqual(['DELETE ?owner=dan'])
})

test('saves the repository’s groups together', async ({ page }) => {
  // Arrange
  const asked = await answersPeople(page)
  await opensPeople(page)
  const choice = page.getByRole('group', { name: 'Groups for acme/widgets' })
  await choice.getByRole('checkbox', { name: '@control-plane-pod' }).check()

  // Act
  await page.getByRole('button', { name: 'Save groups' }).click()

  // Assert
  await expect(page.getByText('Saved the groups for acme/widgets.')).toBeVisible()
  expect(asked).toEqual(['PUT ["S0API","S0POD"]'])
})

test('the mockup links an owner and saves its groups', { tag: '@populated' }, async ({ page }) => {
  // Arrange
  await openCockpit(page, { width: 1440, height }, 'dark')
  await openSection(page, 'Settings')
  await page.getByRole('combobox', { name: 'Slack for ben' }).selectOption({ label: 'Ben Ito' })
  await expect(page.getByText('Saved: ben is Ben Ito.')).toBeVisible()
  await page.getByRole('checkbox', { name: '@web-guild' }).check()

  // Act
  await page.getByRole('button', { name: 'Save groups' }).click()

  // Assert
  await expect(page.getByText('Saved the groups for acme/workflow.')).toBeVisible()
})
