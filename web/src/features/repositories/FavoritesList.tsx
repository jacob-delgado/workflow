import type { Favorite } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'

// stateWords say what is at a favorite now.
const stateWords: Record<Favorite['state'], string> = {
  here: 'Where you work',
  repository: 'Repository',
  directory: 'Not a repository',
  missing: 'Not there any more',
}

interface FavoritesListProps {
  favorites: Favorite[]
  kept: boolean
  onSwitch: (favorite: Favorite) => void
  onForget: (favorite: Favorite) => void
}

// FavoritesList is your favorite directories, each with what is there now, to
// switch to or forget.
export function FavoritesList({ favorites, kept, onSwitch, onForget }: FavoritesListProps) {
  return (
    <section aria-labelledby="favorites" className="flex flex-col gap-item">
      <h2 id="favorites" className="text-base font-semibold">
        Favorites
      </h2>
      {kept ? null : (
        <p className="text-sm text-muted-foreground">
          Favorites are not kept here: the store is turned off, or the server runs with --dry-run.
        </p>
      )}
      {favorites.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No favorites yet. Add the directory you work in, or open another below.
        </p>
      ) : (
        <ul aria-labelledby="favorites" className="flex flex-col divide-y divide-border">
          {favorites.map((favorite) => (
            <FavoriteRow
              key={favorite.dir}
              favorite={favorite}
              kept={kept}
              onSwitch={onSwitch}
              onForget={onForget}
            />
          ))}
        </ul>
      )}
    </section>
  )
}

interface FavoriteRowProps {
  favorite: Favorite
  kept: boolean
  onSwitch: (favorite: Favorite) => void
  onForget: (favorite: Favorite) => void
}

// FavoriteRow is one favorite: where it is, what is there, and what can be
// done with it.
function FavoriteRow({ favorite, kept, onSwitch, onForget }: FavoriteRowProps) {
  const canSwitch = favorite.state === 'repository' || favorite.state === 'directory'

  return (
    <li className="flex flex-wrap items-center justify-between gap-item py-2">
      <span className="flex min-w-0 flex-col">
        <span className="font-mono wrap-anywhere">{favorite.shown}</span>
        <span className="text-sm text-muted-foreground">
          {stateWords[favorite.state]}
          {favorite.origin === '' ? null : (
            <>
              {' on '}
              <span className="font-mono">{favorite.origin}</span>
            </>
          )}
        </span>
      </span>
      <span className="flex gap-item">
        {canSwitch ? (
          <Button
            variant="secondary"
            aria-label={`Switch to ${favorite.shown}`}
            onClick={() => {
              onSwitch(favorite)
            }}
          >
            Switch
          </Button>
        ) : null}
        {kept ? (
          <Button
            variant="secondary"
            aria-label={`Remove ${favorite.shown} from favorites`}
            onClick={() => {
              onForget(favorite)
            }}
          >
            Remove
          </Button>
        ) : null}
      </span>
    </li>
  )
}
