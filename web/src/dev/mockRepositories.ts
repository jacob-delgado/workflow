import type {
  ConfiguredHooks,
  DirectoryListing,
  Favorite,
  Repositories,
  Worktree,
} from '../api/generated/types.gen.ts'

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

// mockWorktrees are api's worktrees: its own, where the mockup works; one for
// a feature git keeps locked; one detached for a review; and one whose
// directory is gone.
const mockWorktrees: Worktree[] = [
  {
    dir: '/home/ana/src/api',
    shown: '~/src/api',
    branch: 'main',
    head: '300a7be',
    state: 'here',
    locked: false,
  },
  {
    dir: '/home/ana/src/api-feat-PROJ-7-rate-limits',
    shown: '~/src/api-feat-PROJ-7-rate-limits',
    branch: 'feat/PROJ-7-rate-limits',
    head: '9c41d02',
    state: 'worktree',
    locked: true,
  },
  {
    dir: '/home/ana/src/api-review',
    shown: '~/src/api-review',
    branch: '',
    head: '5e0b7aa',
    state: 'worktree',
    locked: false,
  },
  {
    dir: '/home/ana/src/api-spike',
    shown: '~/src/api-spike',
    branch: 'spike/cache',
    head: 'd1f8e30',
    state: 'missing',
    locked: false,
  },
]

// mockRepositories is where the mockup works, its worktrees, and its
// favorites.
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
    worktrees: mockWorktrees,
    worktrees_error: '',
    favorites: mockFavorites,
    favorites_kept: true,
  }
}

// subdirectories are the mockup's directories, by the path they are in.
const subdirectories: Record<string, { name: string; repository: boolean }[]> = {
  '/': [{ name: 'home', repository: false }],
  '/home': [{ name: 'ana', repository: false }],
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

// worksIn is where the mockup works, which a relative path starts from.
const worksIn = '/home/ana/src/api/cmd'

// resolved is a path as the server reads it: from your home after ~, from
// where the mockup works when relative, its . and .. walked and no slash at
// its end.
function resolved(typed: string): string {
  const fromHome = typed.startsWith('~') ? `${home}${typed.slice(1)}` : typed
  const absolute = fromHome.startsWith('/') ? fromHome : `${worksIn}/${fromHome}`
  const parts = absolute.split('/').reduce<string[]>((walked, part) => {
    if (part === '..') {
      return walked.slice(0, -1)
    }

    return part === '' || part === '.' ? walked : [...walked, part]
  }, [])

  return `/${parts.join('/')}`
}

// mockDirectories lists a directory of the mockup's, as the server would:
// those whose names start with prefix in any case, all of them for ''.
export function mockDirectories(typed: string, prefix = ''): DirectoryListing {
  const path = resolved(typed)
  const entries = (subdirectories[path] ?? []).filter((entry) =>
    entry.name.toLowerCase().startsWith(prefix.toLowerCase()),
  )
  const parent = path.slice(0, Math.max(path.lastIndexOf('/'), 1))

  return {
    path,
    shown: shown(path),
    parent: path === '/' ? '' : parent,
    entries: entries.map((entry) => ({ ...entry, path: `${path}/${entry.name}` })),
    truncated: false,
  }
}

// mockHooks are the hooks the lefthook of the repository the mockup works in
// configures: pre-commit, pre-push and one of its own a run by hand can run,
// and commit-msg, which only git runs.
export function mockHooks(): ConfiguredHooks {
  return {
    runnable: ['pre-commit', 'pre-push', 'lint-all'],
    git_only: [{ name: 'commit-msg', reason: 'git hands it the file holding the commit message' }],
  }
}
