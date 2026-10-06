import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useHealthStore } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { gitLabWords, makeBranch, makeHealth, makeSnapshot } from '@/test/fixtures.ts'
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

test("on GitLab, names the merge request by GitLab's own mark", async () => {
  // Arrange
  onBranch()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  fakeApi({
    [previewPath]: { key: '42', pull: 9, body: 'Speeds it up.\n\nCloses #42\n', changes: true },
  })
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))
  await user.type(screen.getByRole('textbox', { name: 'Issue' }), '#42')

  // Act
  await user.click(screen.getByRole('button', { name: 'Link' }))

  // Assert
  expect(await screen.findByText("!9's description becomes:")).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Link and update !9' })).toBeTruthy()
})

test('a branch linked to a forge issue names it as the forge writes it', () => {
  // Arrange
  onBranch('42')

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByText('#42')).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Unlink #42' })).toBeTruthy()
})

test('says a forge issue was linked by its number as the forge writes it', async () => {
  // Arrange
  onBranch()
  fakeApi({
    [previewPath]: { key: '42', pull: 0, body: '', changes: false },
    [linkPath]: makeBranch({ name: 'my-thing', issue_link: '42' }),
  })
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))
  await user.type(screen.getByRole('textbox', { name: 'Issue' }), '#42')

  // Act
  await user.click(screen.getByRole('button', { name: 'Link' }))

  // Assert
  expect(await screen.findByText('Linked my-thing to #42.')).toBeTruthy()
})

test('a refused link says why, with focus still on Link', async () => {
  // Arrange
  onBranch()
  fakeApi({
    [previewPath]: () =>
      Response.json({ code: 'unprocessable', detail: 'PROJ-0 is not an issue' }, { status: 422 }),
  })
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))
  await user.type(screen.getByRole('textbox', { name: 'Issue' }), 'PROJ-0')

  // Act
  await user.click(screen.getByRole('button', { name: 'Link' }))

  // Assert
  expect(await screen.findByRole('alert')).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Link' }))
})

test('canceling the link hands focus back to Link an issue', async () => {
  // Arrange
  onBranch()
  fakeApi({})
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Link an issue' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Link an issue' }))
})

test('once linked, shows the link with focus on Unlink before the stream reports it', async () => {
  // Arrange
  onBranch()
  fakeApi({
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
  const unlink = await screen.findByRole('button', { name: 'Unlink PROJ-7' })
  expect(document.activeElement).toBe(unlink)
  expect(screen.queryByRole('button', { name: 'Link an issue' })).toBeNull()
})

test('once unlinked, offers Link an issue with focus before the stream reports it', async () => {
  // Arrange
  onBranch('PROJ-7')
  fakeApi({ [linkPath]: makeBranch({ name: 'my-thing' }) })
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Unlink PROJ-7' }))

  // Assert
  const offer = await screen.findByRole('button', { name: 'Link an issue' })
  expect(document.activeElement).toBe(offer)
  expect(screen.queryByRole('button', { name: 'Unlink PROJ-7' })).toBeNull()
})

test('a refused unlink says so, with focus still on Unlink', async () => {
  // Arrange
  onBranch('PROJ-7')
  fakeApi({ [linkPath]: () => new Response('', { status: 500 }) })
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Unlink PROJ-7' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'The link was not forgotten. Try again.',
  )
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Unlink PROJ-7' }))
})
