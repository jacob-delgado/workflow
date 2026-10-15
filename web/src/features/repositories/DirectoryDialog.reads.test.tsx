import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { z } from 'zod'
import type { DirectoryListing } from '@/api/generated/types.gen.ts'
import { mockRepositories } from '@/dev/mockRepositories.ts'
import { listing, openDialog, options, serving } from '@/test/directoryDialog.ts'
import { fakeApi, held } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { twinCases } from '@/test/twinCases.ts'
import { RepositoriesPanel } from './RepositoriesPanel.tsx'

// How the directory dialog reads a directory, as a user meets it: while it
// reads, when a listing is cut short and asked again by the name typed, when
// a hidden one is asked for by its dot, and when a read is refused.

// The most rows the dialog draws, which internal/tui's picker answers to as
// well.
const { most } = twinCases(
  'testdata/twins/directory_rows.json',
  z.strictObject({ about: z.string(), most: z.number().int().positive() }),
)

// cutShort is what the dialog says of a listing cut short, at any limit.
const cutShort = /^First \d+ shown; keep typing to narrow\.$/

// standing is what the dialog says of the directory listed once it is read:
// how many of its directories match, or that none do, or that it holds none.
const standing = /^(?:\d+ of \d+|\d+ director(?:y|ies)|No directories|Nothing) in /

// big answers for ~/big, a directory of more than a listing holds: cut short
// to first, at as many as it names, and asked again by a prefix, every name
// starting with it in any case, past the first too, as the server answers.
function big(first: string[], past: string[]): (url: URL) => DirectoryListing {
  return (url) => {
    const prefix = url.searchParams.get('prefix')
    const names =
      prefix === null
        ? first
        : [...first, ...past].filter((name) => name.toLowerCase().startsWith(prefix.toLowerCase()))
    const listed = {
      path: '/home/ana/big',
      shown: '~/big',
      parent: '/home/ana',
      entries: names.map((name) => ({ name, path: `/home/ana/big/${name}`, repository: false })),
    }

    return prefix === null ? { ...listed, truncated_to: first.length } : listed
  }
}

test('a directory being read says which', async () => {
  // Arrange
  serving(() => held<DirectoryListing>().promise)
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/src/')

  // Assert
  expect(await screen.findByText('Reading ~/src…')).toBeTruthy()
})

test('a listing cut short says the most the server listed', async () => {
  // Arrange
  serving(big(['api', 'web'], ['webhooks']))
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/big/')

  // Assert
  expect(await screen.findByText('First 2 shown; keep typing to narrow.')).toBeTruthy()
})

test('a listing cut short is asked again for the names starting with what was typed', async () => {
  // Arrange
  serving(big(['api', 'web'], ['webhooks']))
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/big/we')

  // Assert
  await waitFor(() => {
    expect(options()).toEqual(['web', 'webhooks', 'switch here~/big'])
  })
  expect(screen.queryByText(cutShort)).toBeNull()
})

test('in a listing cut short, a match the first listing drew stays once it is asked again', async () => {
  // Arrange
  serving(big(['Workday', 'two-words', 'other'], ['world-beyond']))
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/big/wo')

  // Assert
  await waitFor(() => {
    expect(options()).toEqual(['Workday', 'world-beyond', 'two-words', 'switch here~/big'])
  })
})

test('in a listing cut short, what was found for the name typed stays while the next is read', async () => {
  // Arrange
  const answer = big(['Workday', 'two-words', 'other'], ['world-beyond'])
  serving((url) =>
    url.searchParams.get('prefix') === 'wor' ? held<DirectoryListing>().promise : answer(url),
  )
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/big/wo')
  await waitFor(() => {
    expect(options()).toContain('world-beyond')
  })

  // Act
  await user.type(field, 'r')

  // Assert
  expect(options()).toEqual(['Workday', 'world-beyond', 'two-words', 'switch here~/big'])
  expect(screen.queryByText(cutShort)).toBeNull()
})

test('a listing cut short that cannot be asked again is an alert in the dialog', async () => {
  // Arrange
  const answer = big(['api', 'web'], [])
  serving((url) =>
    url.searchParams.get('prefix') === null ? answer(url) : new Response('', { status: 500 }),
  )
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/big/we')

  // Assert
  const dialog = screen.getByRole('dialog', { name: 'Go to a directory' })
  expect(await within(dialog).findByRole('alert')).toBeTruthy()
})

test('a hidden directory is asked for once its dot is typed', async () => {
  // Arrange
  // The server lists a hidden directory only for a prefix starting with its
  // dot.
  serving((url) => {
    const listed = listing(url)
    const prefix = url.searchParams.get('prefix') ?? ''
    const hidden = { name: '.config', path: '/home/ana/.config', repository: false }

    return prefix.startsWith('.') && hidden.name.startsWith(prefix)
      ? { ...listed, entries: [hidden] }
      : listed
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/.conf')

  // Assert
  await waitFor(() => {
    expect(options()).toEqual(['.config', 'switch here~'])
  })
})

test.each([
  ['nothing', [], ['switch here~/src']],
  ['a hidden directory', ['.cache'], ['.cache', 'switch here~/src']],
])(
  'a listing asked again by its dot that answers before the whole one, finding %s, leaves the dialog reading',
  async (_, hidden, listed) => {
    // Arrange
    // The whole listing of ~/src is held back, and the one narrowed by the
    // dot answers: what the whole one holds may match the dot too.
    serving((url) => {
      const prefix = url.searchParams.get('prefix')
      if (url.searchParams.get('path') === '~/src' && prefix === null) {
        return held<DirectoryListing>().promise
      }

      const entries = hidden.map((name) => ({
        name,
        path: `/home/ana/src/${name}`,
        repository: false,
      }))

      return prefix === '.' ? { ...listing(url), entries } : listing(url)
    })
    const user = userEvent.setup()
    renderWithClient(<RepositoriesPanel />)
    const field = await openDialog(user)

    // Act
    await user.type(field, '~/src/.')

    // Assert
    await waitFor(() => {
      expect(options()).toEqual(listed)
    })
    expect(screen.getByText('Reading ~/src…')).toBeTruthy()
    expect(screen.queryByText(standing)).toBeNull()
  },
)

test('the whole listing answering after the one asked again by its dot counts what both hold', async () => {
  // Arrange
  // The whole listing of ~/src is held back while the one narrowed by the
  // dot answers with .cache; the whole one then holds x.y, which matches the
  // dot letter by letter, and api, which does not.
  const whole = held<DirectoryListing>()
  serving((url) => {
    const prefix = url.searchParams.get('prefix')
    if (url.searchParams.get('path') === '~/src' && prefix === null) {
      return whole.promise
    }

    const cache = { name: '.cache', path: '/home/ana/src/.cache', repository: false }

    return prefix === '.' ? { ...listing(url), entries: [cache] } : listing(url)
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/src/.')
  await waitFor(() => {
    expect(options()).toEqual(['.cache', 'switch here~/src'])
  })

  // Act
  whole.answer({
    path: '/home/ana/src',
    shown: '~/src',
    parent: '/home/ana',
    entries: ['api', 'x.y'].map((name) => ({
      name,
      path: `/home/ana/src/${name}`,
      repository: false,
    })),
  })

  // Assert
  expect(await screen.findByText('2 of 3 in ~/src')).toBeTruthy()
  expect(screen.queryByText('Reading ~/src…')).toBeNull()
})

test('a listing asked again by its dot that is refused while the whole one is read says both', async () => {
  // Arrange
  // The whole listing of ~/src is held back, and the one narrowed by the dot
  // is refused.
  serving((url) => {
    const prefix = url.searchParams.get('prefix')
    if (url.searchParams.get('path') === '~/src' && prefix === null) {
      return held<DirectoryListing>().promise
    }

    return prefix === '.' ? new Response('', { status: 500 }) : listing(url)
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/src/.')

  // Assert
  const dialog = screen.getByRole('dialog', { name: 'Go to a directory' })
  expect(await within(dialog).findByRole('alert')).toBeTruthy()
  const said = within(dialog).getAllByRole('status')
  expect(said.map((line) => line.textContent)).toContain('Reading ~/src…')
})

test('at most the matches the shared file names are listed, with how many more there are', async () => {
  // Arrange
  const many = Array.from({ length: most + 41 }, (_, index) => ({
    name: `dir${String(index).padStart(3, '0')}`,
    path: `/home/ana/src/dir${String(index)}`,
    repository: false,
  }))
  serving((url) => ({ ...listing(url), entries: many }))
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/src/d')

  // Assert
  expect(await screen.findByText('41 more match; keep typing to narrow.')).toBeTruthy()
  expect(options()).toHaveLength(most + 1)
})

test('a directory that cannot be listed is an alert in the dialog, which stays open', async () => {
  // Arrange
  fakeApi({
    '/api/repositories': mockRepositories(),
    '/api/directories': () => new Response('', { status: 500 }),
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await openDialog(user)

  // Assert
  const dialog = screen.getByRole('dialog', { name: 'Go to a directory' })
  expect(await within(dialog).findByRole('alert')).toBeTruthy()
})

test.each([
  [
    'still being read',
    () => held<DirectoryListing>().promise,
    () => screen.findByText('Reading ~/src/api…'),
  ],
  ['refused', () => new Response('', { status: 404 }), () => screen.findByRole('alert')],
])(
  'Backspace after the slash of a directory %s goes up the path typed',
  async (_, answer, shown) => {
    // Arrange
    serving((url) => (url.searchParams.get('path') === '~/src/api' ? answer() : listing(url)))
    const user = userEvent.setup()
    renderWithClient(<RepositoriesPanel />)
    const field = await openDialog(user)
    await user.type(field, '~/src/api/')
    await shown()

    // Act
    await user.keyboard('{Backspace}')

    // Assert
    expect(field).toHaveProperty('value', '~/src/')
  },
)
