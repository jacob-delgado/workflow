import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Repositories } from '@/api/generated/types.gen.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { Failure, Reading, Unread } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, contentMeasure } from '@/lib/utils.ts'
import { ConfirmSwitch, type Destination } from './ConfirmSwitch.tsx'
import { DirectoryPicker } from './DirectoryPicker.tsx'
import { FavoritesList } from './FavoritesList.tsx'
import { useFavorite, useRepositories, useSwitchTo } from './repositoriesApi.ts'
import { WorkingIn } from './WorkingIn.tsx'
import { WorktreesList } from './WorktreesList.tsx'

// RepositoriesPanel is where the server works, relative to what it changes;
// the repository's worktrees; your favorite directories; and a picker to open another, each switch asked
// once more before it is made, since every section is read again after it.
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
  const [destination, setDestination] = useState<Destination | null>(null)
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
  const favoriteKeys = useShortcutProps<HTMLButtonElement>('favorite-directory')

  return (
    <div className={cn('flex flex-col gap-section', contentMeasure)}>
      <div className="flex flex-col gap-item">
        <WorkingIn place={here} />
        {repositories.favorites_kept ? (
          <div>
            <Button
              {...favoriteKeys}
              variant="secondary"
              onClick={() => {
                void (hereKept === undefined
                  ? toggle.run(here.dir, here.shown, true)
                  : toggle.run(hereKept.dir, hereKept.shown, false))
              }}
            >
              {hereKept === undefined ? 'Add to favorites' : 'Remove from favorites'}
            </Button>
          </div>
        ) : null}
        <OutcomeLine said={outcome.said} />
        {toggle.state === 'error' ? <Failure>{toggle.error}</Failure> : null}
      </div>
      {destination === null ? null : (
        <ConfirmSwitch
          destination={destination}
          switchTo={switchTo}
          teller={outcome}
          onCancel={() => {
            setDestination(null)
          }}
        />
      )}
      <WorktreesList
        worktrees={repositories.worktrees}
        error={repositories.worktrees_error}
        onSwitch={(chosen) => {
          setDestination({ dir: chosen.dir, shown: chosen.shown })
        }}
      />
      <FavoritesList
        favorites={repositories.favorites}
        kept={repositories.favorites_kept}
        onSwitch={(chosen) => {
          setDestination({ dir: chosen.dir, shown: chosen.shown })
        }}
        onForget={(chosen) => {
          void toggle.run(chosen.dir, chosen.shown, false)
        }}
      />
      <DirectoryPicker
        onChoose={(dir, shown) => {
          setDestination({ dir, shown })
        }}
      />
    </div>
  )
}
