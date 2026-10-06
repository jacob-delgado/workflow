import { useEffect, useRef, useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Repositories } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { Failure, Reading, Unread } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { DirectoryPicker } from './DirectoryPicker.tsx'
import { FavoritesList } from './FavoritesList.tsx'
import { useFavorite, useRepositories, useSwitchTo } from './repositoriesApi.ts'
import { WorkingIn } from './WorkingIn.tsx'
import { WorktreesList } from './WorktreesList.tsx'

// Destination is a directory a switch was asked for, before it is confirmed.
interface Destination {
  dir: string
  shown: string
}

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

  return (
    <div className="flex max-w-prose flex-col gap-section">
      <div className="flex flex-col gap-item">
        <WorkingIn place={here} />
        {repositories.favorites_kept ? (
          <div>
            <Button
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

interface ConfirmSwitchProps {
  destination: Destination
  switchTo: (dir: string) => Promise<Repositories>
  teller: Teller
  onCancel: () => void
}

// ConfirmSwitch asks once more before a switch, and makes it. It takes the
// focus as it opens, and is scrolled to: a switch asked from the picker, at
// the foot of the section, would otherwise ask out of sight.
function ConfirmSwitch({ destination, switchTo, teller, onCancel }: ConfirmSwitchProps) {
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    heading.current?.focus()
  }, [destination.dir])
  const go = useAsyncAction(() => switchTo(destination.dir), {
    fallback: 'The directory could not be switched to.',
    done: (switched) => `Switched to ${switched.here.shown}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <section
      aria-labelledby="confirm-switch"
      className="flex flex-col gap-item rounded-lg border border-border p-4"
    >
      <h2
        id="confirm-switch"
        ref={heading}
        tabIndex={-1}
        className="font-semibold focus-visible:outline-none"
      >
        Switch to <span className="font-mono">{destination.shown}</span>?
      </h2>
      <p className="text-sm text-muted-foreground">Every section is read again there.</p>
      {go.state === 'error' ? <Failure>{go.error}</Failure> : null}
      <div className="flex gap-item">
        <Button
          variant="primary"
          aria-disabled={go.state === 'running'}
          onClick={() => {
            if (go.state !== 'running') {
              void go.run()
            }
          }}
        >
          Switch
        </Button>
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </section>
  )
}
