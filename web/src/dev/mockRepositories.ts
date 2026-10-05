import type { DirectoryListing, Favorite, Repositories } from '../api/generated/types.gen.ts'

// The mockup's directories, for `task web:mockup` and the end-to-end specs:
// api worked in from its cmd directory, with web, the notes and a removed
// directory kept as favorites. Relative imports, since the e2e build reads
// this file without the app's path aliases.

const home = '/home/ana'

// shown is a directory written from the mockup's home.
function shown(dir: string): string {
  return dir === home ? '~' : dir.replace(`${home}/`, '~/')
}

// mockFavorites are the favorites the mockup keeps, by path.
const mockFavorites: Favorite[] = [
  {
    dir: '/home/ana/src/web',
    shown: '~/src/web',
    state: 'repository',
    origin: 'github.com/acme/web',
  },
  { dir: '/home/ana/notes', shown: '~/notes', state: 'directory', origin: '' },
  { dir: '/home/ana/old-site', shown: '~/old-site', state: 'missing', origin: '' },
]

// mockRepositories is where the mockup works, and its favorites.
export function mockRepositories(): Repositories {
  return {
    here: {
      dir: '/home/ana/src/api/cmd',
      shown: '~/src/api/cmd',
      root: '/home/ana/src/api',
      root_shown: '~/src/api',
      within: 'cmd',
      origin: 'github.com/acme/api',
      config: ['~/src/api/.workflow.json', '~/.workflow.json'],
    },
    favorites: mockFavorites,
    favorites_kept: true,
  }
}

// subdirectories are the mockup's directories, by the path they are in.
const subdirectories: Record<string, { name: string; repository: boolean }[]> = {
  '/home/ana': [
    { name: 'notes', repository: false },
    { name: 'src', repository: false },
  ],
  '/home/ana/src': [
    { name: 'api', repository: true },
    { name: 'web', repository: true },
  ],
  '/home/ana/src/api': [{ name: 'cmd', repository: false }],
}

// mockDirectories lists a directory of the mockup's, as the server would.
export function mockDirectories(path: string): DirectoryListing {
  const entries = subdirectories[path] ?? []
  const parent = path.slice(0, Math.max(path.lastIndexOf('/'), 1))

  return {
    path,
    shown: shown(path),
    parent: path === '/' ? '' : parent,
    entries: entries.map((entry) => ({ ...entry, path: `${path}/${entry.name}` })),
    truncated: false,
  }
}
