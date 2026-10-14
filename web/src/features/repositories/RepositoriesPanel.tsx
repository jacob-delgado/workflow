import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Repositories } from '@/api/generated/types.gen.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { Failure, Reading, Unread } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, contentMeasure } from '@/lib/utils.ts'
import type { Destination } from './ConfirmSwitch.tsx'
import { DirectoryDialog } from './DirectoryDialog.tsx'
import { FavoritesList } from './FavoritesList.tsx'
import { useFavorite, useRepositories, useSwitchTo } from './repositoriesApi.ts'
import { WorkingIn } from './WorkingIn.tsx'
import { WorktreesList } from './WorktreesList.tsx'

// RepositoriesPanel is where the server works, relative to what it changes;
// the repository's worktrees; your favorite directories; and a dialog to go to
// another, each switch asked once more in it before it is made, since every
// section is read again after it.
export function RepositoriesPanel() {
  const read = useRepositories()
  const outcome = useOutcome()

  if (read.data === undefined) {
    return read.isError ? (
      <Unread
        reason={apiErrorMessage(read.error, 'Where the server works could not be read.')}
        refusals={read.errorUpdateCount}
        retrying={read.isFetching}
        onRetry={() => {
          void read.refetch()
        }}
      />
    ) : (
      <Reading>Reading where the server works…</Reading>
    )
  }

  return <Read repositories={read.data} outcome={outcome} />
}

// Read is the section once the server has said where it works.
function Read({
  repositories,
  outcome,
}: {
  repositories: Repositories
  outcome: ReturnType<typeof useOutcome>
}) {
  // going is the directory dialog while it is open, and the switch it opens on
  // asking, where one was asked for from a worktree or a favorite.
  const [going, setGoing] = useState<{ asking: Destination | null } | null>(null)
  const favorite = useFavorite()
  const switchTo = useSwitchTo()
  const toggle = useAsyncAction(
    (dir: string, _shown: string, keep: boolean) => favorite(dir, keep),
    {
      fallback: 'The favorite could not be changed.',
      done: (_, __, shown, keep) =>
        keep ? `Added ${shown} to favorites.` : `Removed ${shown} from favorites.`,
      onStart: outcome.clear,
      onDone: outcome.say,
    },
  )
  const here = repositories.here
  // The favorite that is where the server works, by the name it was kept
  // under, which may reach it through a link: the name to forget.
  const hereKept = repositories.favorites.find((each) => each.state === 'here')

  return (
    <div className={cn('flex flex-col gap-section', contentMeasure)}>
      <div className="flex flex-col gap-item">
        <WorkingIn place={here} />
        <HereActions
          kept={repositories.favorites_kept}
          hereKept={hereKept !== undefined}
          onGo={() => {
            setGoing({ asking: null })
          }}
          onFavorite={() => {
            void (hereKept === undefined
              ? toggle.run(here.dir, here.shown, true)
              : toggle.run(hereKept.dir, hereKept.shown, false))
          }}
        />
        <OutcomeLine said={outcome.said} />
        {toggle.state === 'error' ? <Failure>{toggle.error}</Failure> : null}
      </div>
      {going === null ? null : (
        <DirectoryDialog
          repositories={repositories}
          asking={going.asking}
          switchTo={switchTo}
          teller={outcome}
          onClose={() => {
            setGoing(null)
          }}
        />
      )}
      <WorktreesList
        worktrees={repositories.worktrees}
        error={repositories.worktrees_error}
        onSwitch={(chosen) => {
          setGoing({ asking: { dir: chosen.dir, shown: chosen.shown } })
        }}
      />
      <FavoritesList
        favorites={repositories.favorites}
        kept={repositories.favorites_kept}
        onSwitch={(chosen) => {
          setGoing({ asking: { dir: chosen.dir, shown: chosen.shown } })
        }}
        onForget={(chosen) => {
          void toggle.run(chosen.dir, chosen.shown, false)
        }}
      />
    </div>
  )
}

interface HereActionsProps {
  // kept is whether the server keeps favorites, and hereKept whether where it
  // works is one.
  kept: boolean
  hereKept: boolean
  onGo: () => void
  onFavorite: () => void
}

// HereActions are what can be done from where the server works: go to another
// directory, and keep this one as a favorite or forget it.
function HereActions({ kept, hereKept, onGo, onFavorite }: HereActionsProps) {
  const favoriteKeys = useShortcutProps<HTMLButtonElement>('favorite-directory')
  const goKeys = useShortcutProps<HTMLButtonElement>('go-to-directory')

  return (
    <div className="flex flex-wrap gap-item">
      <Button {...goKeys} variant="secondary" onClick={onGo}>
        Go to a directory…
      </Button>
      {kept ? (
        <Button {...favoriteKeys} variant="secondary" onClick={onFavorite}>
          {hereKept ? 'Remove from favorites' : 'Add to favorites'}
        </Button>
      ) : null}
    </div>
  )
}
