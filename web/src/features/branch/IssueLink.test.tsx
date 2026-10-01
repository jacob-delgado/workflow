import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'

const linkPath = '/api/branch/issue'
const previewPath = '/api/branch/issue/preview'

// onBranch streams a branch begun outside workflow, linked to link when given.
function onBranch(link = '') {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ branch: makeBranch({ name: 'my-thing', issue_link: link }) }),
  })
}

// writesTo are the requests that changed the link, as "METHOD body".
async function writesTo(requests: Request[]): Promise<string[]> {
  const writes = requests.filter(
    (request) => new URL(request.url).pathname === linkPath && request.method !== 'GET',
  )

  return Promise.all(
    writes.map(async (request) => `${request.method} ${await request.clone().text()}`),
  )
}

test('links the branch to the issue typed when no description would change', async () => {
  // Arrange
  onBranch()
  const requests = fakeApi({
    [previewPath]: { key: 'PROJ-7', pull: 0, body: '', changes: false },
    [linkPath]: makeBranch({ name: 'my-thing', issue_link: 'PROJ-7' }),
  })
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))
  await user.type(screen.getByRole('textbox', { name: 'Issue' }), 'PROJ-7')

  // Act
  await user.click(screen.getByRole('button', { name: 'Link' }))

  // Assert
  expect(await screen.findByText('Linked my-thing to PROJ-7.')).toBeTruthy()
  expect(await writesTo(requests)).toEqual(['PUT {"key":"PROJ-7","update_pull":false}'])
})

test('with a pull request, shows its new description before linking', async () => {
  // Arrange
  onBranch()
  const requests = fakeApi({
    [previewPath]: { key: '42', pull: 9, body: 'Speeds it up.\n\nCloses #42\n', changes: true },
    [linkPath]: makeBranch({ name: 'my-thing', issue_link: '42' }),
  })
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))
  await user.type(screen.getByRole('textbox', { name: 'Issue' }), '#42')

  // Act
  await user.click(screen.getByRole('button', { name: 'Link' }))

  // Assert
  expect(await screen.findByText(/Closes #42/)).toBeTruthy()
  expect(await writesTo(requests)).toEqual([])
})

test('linking and updating sends the description change with the link', async () => {
  // Arrange
  onBranch()
  const requests = fakeApi({
    [previewPath]: { key: '42', pull: 9, body: 'Speeds it up.\n\nCloses #42\n', changes: true },
    [linkPath]: makeBranch({ name: 'my-thing', issue_link: '42' }),
  })
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))
  await user.type(screen.getByRole('textbox', { name: 'Issue' }), '#42')
  await user.click(screen.getByRole('button', { name: 'Link' }))

  // Act
  await user.click(await screen.findByRole('button', { name: /link and update #9/i }))

  // Assert
  await waitFor(async () => {
    expect(await writesTo(requests)).toEqual(['PUT {"key":"#42","update_pull":true}'])
  })
})

test('a linked branch names its issue and can be unlinked', async () => {
  // Arrange
  onBranch('PROJ-7')
  const requests = fakeApi({ [linkPath]: makeBranch({ name: 'my-thing' }) })
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Unlink PROJ-7' }))

  // Assert
  await waitFor(async () => {
    expect(await writesTo(requests)).toEqual(['DELETE '])
  })
})

test('links the issue that was previewed, not one typed while the preview was read', async () => {
  // Arrange
  onBranch()
  let answerPreview = () => {}
  const previewHeld = new Promise<void>((resolve) => {
    answerPreview = resolve
  })
  const requests = fakeApi({
    [previewPath]: async () => {
      await previewHeld

      return { key: 'PROJ-7', pull: 12, body: 'Speeds it up.\n\nJira: PROJ-7\n', changes: true }
    },
    [linkPath]: makeBranch({ name: 'my-thing', issue_link: 'PROJ-7' }),
  })
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))
  const field = screen.getByRole('textbox', { name: 'Issue' })
  await user.type(field, 'PROJ-7{Enter}')
  await user.type(field, '{Backspace}9')
  answerPreview()

  // Act
  await user.click(await screen.findByRole('button', { name: 'Link and update #12' }))

  // Assert
  await waitFor(async () => {
    expect(await writesTo(requests)).toEqual(['PUT {"key":"PROJ-7","update_pull":true}'])
  })
})
