import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { People } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockDirectories, mockRepositories } from '@/dev/mockRepositories.ts'
import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { RepositoriesPanel } from '@/features/repositories/RepositoriesPanel.tsx'
import { PeopleAndGroups } from '@/features/settings/people/PeopleAndGroups.tsx'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'

// Every write's button says it is busy, in the verb it rests in, while its
// request is out: each route here holds its answer for as long as the test runs.

// held never answers.
function held(): Promise<never> {
  return new Promise(() => undefined)
}

// onBranch streams the branch my-thing, linked to link, with the changes given.
function onBranch(link: string, staged: boolean[] = []) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({ name: 'my-thing', issue_link: link }),
      changes: {
        changes: staged.map((isStaged, place) => ({
          path: `file${String(place)}.go`,
          kind: 'modified',
          staged: isStaged,
          has_unstaged: !isStaged,
          conflicted: false,
        })),
      },
    }),
  })
}

test('Switch says Switching… while the switch is out', async () => {
  // Arrange
  fakeApi({
    '/api/repositories': mockRepositories(),
    '/api/repositories/here': held,
    '/api/directories': mockDirectories('/home/ana/src/api/cmd'),
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  await user.click(await screen.findByRole('button', { name: 'Switch to ~/src/web' }))
  const question = screen.getByRole('region', { name: 'Switch to ~/src/web?' })

  // Act
  await user.click(within(question).getByRole('button', { name: 'Switch' }))

  // Assert
  expect(await within(question).findByRole('button', { name: 'Switching…' })).toBeTruthy()
})

test.each([
  { name: 'Stage file0.go', staged: false, busy: 'Staging…' },
  { name: 'Unstage file0.go', staged: true, busy: 'Unstaging…' },
])('$name says $busy while its request is out', async ({ name, staged, busy }) => {
  // Arrange
  onBranch('', [staged])
  fakeApi({ '/api/stage': held, '/api/unstage': held })
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name }))

  // Assert
  expect((await screen.findByRole('button', { name: new RegExp(`^${busy}`) })).textContent).toBe(
    busy,
  )
})

test('Unlink says Unlinking… while its request is out', async () => {
  // Arrange
  onBranch('PROJ-7')
  fakeApi({ '/api/branch/issue': held })
  const user = userEvent.setup()
  render(<BranchPanel />)
  const unlink = screen.getByRole('button', { name: 'Unlink PROJ-7' })

  // Act
  await user.click(unlink)

  // Assert
  expect(unlink.textContent).toBe('Unlinking…')
})

test('Link says Linking… while its request is out', async () => {
  // Arrange
  onBranch('')
  fakeApi({ '/api/branch/issue/preview': held })
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))
  await user.type(screen.getByRole('textbox', { name: 'Issue' }), 'PROJ-7')

  // Act
  await user.click(screen.getByRole('button', { name: 'Link' }))

  // Assert
  expect(await screen.findByRole('button', { name: 'Linking…' })).toBeTruthy()
})

test('Forget says Forgetting… while its request is out', async () => {
  // Arrange
  const people: People = { owners: [{ owner: 'dan', kind: 'user', state: 'not_on_slack' }] }
  fakeApi({
    '/api/people': (_: URL, asked: Request) => (asked.method === 'DELETE' ? held() : people),
    '/api/repo-groups': { repository: 'acme/widgets', groups: [] },
    '/api/slack/members': { entries: [] },
    '/api/slack/groups': { entries: [] },
  })
  const user = userEvent.setup()
  renderWithClient(<PeopleAndGroups />)
  await user.click(await screen.findByRole('button', { name: 'Forget dan…' }))
  const question = screen.getByRole('group', { name: 'Forget dan?' })

  // Act
  await user.click(within(question).getByRole('button', { name: 'Forget' }))

  // Assert
  expect(await within(question).findByRole('button', { name: 'Forgetting…' })).toBeTruthy()
})
