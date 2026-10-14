import { screen, waitFor, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import type { DirectoryListing } from '@/api/generated/types.gen.ts'
import { mockDirectories, mockRepositories } from '@/dev/mockRepositories.ts'
import { fakeApi } from './fakeApi.ts'

// The directory dialog as its cases meet it in Repositories: the section's
// reads, the dialog opened, and what it lists. DirectoryDialog.test.tsx and
// its .reads sibling share it.

// DirectoryAnswer is how a directory read is answered: a listing, a refusal,
// or either held back.
type DirectoryAnswer = DirectoryListing | Response | Promise<DirectoryListing | Response>

// listing answers a directory read as the mockup does, by its path, or by
// where the mockup works, and its prefix.
export function listing(url: URL): DirectoryListing {
  return mockDirectories(url.searchParams.get('path') ?? '.', url.searchParams.get('prefix') ?? '')
}

// serving answers the section's reads, the directory reads by listing unless
// directories answers them, and a switch with web as where the server works.
export function serving(directories: (url: URL) => DirectoryAnswer = listing): Request[] {
  const repositories = mockRepositories()

  return fakeApi({
    '/api/repositories': repositories,
    '/api/repositories/here': {
      ...repositories,
      here: { ...repositories.here, dir: '/home/ana/src/web', shown: '~/src/web' },
    },
    '/api/directories': directories,
  })
}

// openDialog opens the directory dialog and waits for its field to have the
// focus.
export async function openDialog(user: UserEvent): Promise<HTMLElement> {
  await user.click(await screen.findByRole('button', { name: 'Go to a directory…' }))
  const field = await screen.findByRole('combobox', { name: 'Directory' })
  await waitFor(() => {
    expect(document.activeElement).toBe(field)
  })

  return field
}

// options is the text of each option the dialog lists, in order.
export function options(): (string | null)[] {
  return within(screen.getByRole('dialog', { name: 'Go to a directory' }))
    .getAllByRole('option')
    .map((option) => option.textContent)
}
