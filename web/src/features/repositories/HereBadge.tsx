import { FolderGit2 } from 'lucide-react'
import { useUiStore } from '@/shell/uiStore.ts'
import { useRepositories } from './repositoriesApi.ts'

// HereBadge says in the header where the server works — the repository's
// name and the path within it, as the terminal's top row does — and opens
// the Repositories section, which says it in full.
export function HereBadge() {
  const read = useRepositories()
  const setSection = useUiStore((state) => state.setSection)
  const here = read.data?.here
  if (here === undefined) {
    return null
  }

  const name = here.root === '' ? baseName(here.dir) : baseName(here.root)
  const label = here.within === '' ? name : `${name}/${here.within}`

  return (
    <button
      type="button"
      aria-label={`Working in ${here.shown}: open Repositories`}
      onClick={() => {
        setSection('repositories')
      }}
      className="flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1 font-mono text-sm text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      <FolderGit2 aria-hidden className="size-4 shrink-0" />
      <span className="truncate">{label}</span>
    </button>
  )
}

// baseName is a path's last part, after either separator, so a Windows
// path is named too.
function baseName(path: string): string {
  return path.slice(Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1) || path
}
