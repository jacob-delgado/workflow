import type { Worktree } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { Failure } from '@/lib/Status.tsx'

interface WorktreesListProps {
  worktrees: Worktree[]
  error: string
  onSwitch: (worktree: Worktree) => void
}

// WorktreesList is the working trees of the repository the server works in,
// each with what it has checked out, to switch to. Outside a repository, with
// none to list and nothing gone wrong, it is not drawn.
export function WorktreesList({ worktrees, error, onSwitch }: WorktreesListProps) {
  if (worktrees.length === 0 && error === '') {
    return null
  }

  return (
    <section aria-labelledby="worktrees" className="flex flex-col gap-item">
      <h2 id="worktrees" className="text-lg font-semibold">
        Worktrees
      </h2>
      {error === '' ? null : <Failure>{`The worktrees could not be read: ${error}`}</Failure>}
      {worktrees.length === 0 ? null : (
        <ul aria-labelledby="worktrees" className="flex flex-col divide-y divide-border">
          {worktrees.map((worktree) => (
            <WorktreeRow key={worktree.dir} worktree={worktree} onSwitch={onSwitch} />
          ))}
        </ul>
      )}
    </section>
  )
}

// CheckedOut says what a worktree has checked out — its branch, or the commit
// its HEAD is detached at, each an identifier so set in monospace — and
// whether git keeps it locked; or that its directory is gone.
function CheckedOut({ worktree }: { worktree: Worktree }) {
  if (worktree.state === 'missing') {
    return <>Not there any more</>
  }

  const here = worktree.state === 'here'
  const on =
    worktree.branch === '' ? (
      <>
        {here ? 'at ' : 'At '}
        <span className="font-mono">{worktree.head}</span>, detached
      </>
    ) : (
      <>
        {here ? 'on ' : 'On '}
        <span className="font-mono">{worktree.branch}</span>
      </>
    )

  return (
    <>
      {here ? 'Where you work, ' : null}
      {on}
      {worktree.locked ? ', locked' : null}
    </>
  )
}

// WorktreeRow is one worktree: where it is, what it has checked out, and a
// switch to it when it is another one still there.
function WorktreeRow({
  worktree,
  onSwitch,
}: {
  worktree: Worktree
  onSwitch: (worktree: Worktree) => void
}) {
  return (
    <li className="flex flex-wrap items-center justify-between gap-item py-2">
      <span className="flex min-w-0 flex-col">
        <span className="font-mono wrap-anywhere">{worktree.shown}</span>
        <span className="text-sm text-muted-foreground">
          <CheckedOut worktree={worktree} />
        </span>
      </span>
      {worktree.state === 'worktree' ? (
        <Button
          variant="secondary"
          aria-label={`Switch to ${worktree.shown}`}
          onClick={() => {
            onSwitch(worktree)
          }}
        >
          Switch
        </Button>
      ) : null}
    </li>
  )
}
