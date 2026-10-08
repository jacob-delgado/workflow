import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useHealthStore } from '@/api/health.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeHealth } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { LocalData } from './LocalData.tsx'
import { PeopleAndGroups } from './PeopleAndGroups.tsx'

// refusing refuses every read of path with a problem naming it, and answers
// the other reads People and groups makes with nothing.
function refusing(path: string): void {
  fakeApi({
    '/api/people': { owners: [] },
    '/api/repo-groups': { repository: 'acme/widgets', groups: [] },
    '/api/slack/groups': { entries: [] },
    '/api/slack/members': { entries: [] },
    '/api/local-data': {
      dir: '/home/ana/.local/state/workflow',
      files: [],
      consequences: { cache: '', all: '' },
    },
    [path]: () =>
      Response.json(
        {
          type: 'about:blank',
          title: 'Bad gateway',
          status: 502,
          code: 'unreachable',
          detail: `${path} refused`,
        },
        { status: 502 },
      ),
  })
}

beforeEach(() => {
  useHealthStore.setState({ health: makeHealth() })
})

test.each([
  ['the code owners', '/api/people'],
  ['the repository’s groups', '/api/repo-groups'],
])('%s that cannot be read are an alert beside Try again', async (_, path) => {
  // Arrange
  refusing(path)

  // Act
  renderWithClient(<PeopleAndGroups />)

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toBe(`${path} refused`)
  const around = alert.parentElement ?? document.body
  expect(within(around).getByRole('button', { name: 'Try again' })).toBeTruthy()
})

test('the local data that cannot be read is an alert, read again on Try again', async () => {
  // Arrange
  refusing('/api/local-data')
  const user = userEvent.setup()
  renderWithClient(<LocalData />)
  const again = await screen.findByRole('button', { name: 'Try again' })
  expect(screen.getByRole('alert').textContent).toBe('/api/local-data refused')
  fakeApi({
    '/api/local-data': {
      dir: '/home/ana/.local/state/workflow',
      files: [],
      consequences: { cache: '', all: '' },
    },
  })

  // Act
  await user.click(again)

  // Assert
  expect(await screen.findByText(/No local data/)).toBeTruthy()
})
