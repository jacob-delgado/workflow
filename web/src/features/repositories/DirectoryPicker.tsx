import { useId, useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { DirectoryListing } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { useDirectories } from './repositoriesApi.ts'

// DirectoryPicker browses for a directory to switch to: a path typed — from
// where the server works, or from your home after ~ — or a directory opened
// from the list of those in the one shown, and up to the one above it.
export function DirectoryPicker({ onChoose }: { onChoose: (dir: string, shown: string) => void }) {
  const [path, setPath] = useState('')
  const [typed, setTyped] = useState('')
  const listing = useDirectories(path)
  const inputId = useId()

  return (
    <section aria-labelledby="open-another" className="flex flex-col gap-item">
      <h2 id="open-another" className="text-lg font-semibold">
        Open another directory
      </h2>
      <form
        className="flex flex-wrap items-end gap-item"
        onSubmit={(event) => {
          event.preventDefault()
          setPath(typed.trim())
        }}
      >
        <label
          htmlFor={inputId}
          className="flex min-w-0 flex-1 flex-col gap-tight text-sm text-muted-foreground"
        >
          Directory
          <input
            id={inputId}
            value={typed}
            placeholder={listing.data?.shown ?? ''}
            onChange={(event) => {
              setTyped(event.target.value)
            }}
            className="min-w-0 rounded-md border border-input bg-background px-2 py-1 font-mono text-sm text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
          />
        </label>
        <Button type="submit" variant="secondary">
          Show
        </Button>
      </form>
      {listing.isError ? (
        <p className="text-sm text-foreground">
          {apiErrorMessage(listing.error, 'That directory could not be listed.')}
        </p>
      ) : null}
      {listing.data === undefined ? null : (
        <Listing
          listing={listing.data}
          onOpen={(dir) => {
            setPath(dir)
            setTyped('')
          }}
          onChoose={onChoose}
        />
      )}
    </section>
  )
}

interface ListingProps {
  listing: DirectoryListing
  onOpen: (dir: string) => void
  onChoose: (dir: string, shown: string) => void
}

// Listing is the directory shown, to switch to or leave for the one above,
// and the directories in it, each opened by its name.
function Listing({ listing, onOpen, onChoose }: ListingProps) {
  return (
    <div className="flex flex-col gap-item">
      <div className="flex flex-wrap items-center gap-item">
        <span className="min-w-0 flex-1 font-mono wrap-anywhere">{listing.shown}</span>
        {listing.parent === '' ? null : (
          <Button
            variant="secondary"
            onClick={() => {
              onOpen(listing.parent)
            }}
          >
            Up
          </Button>
        )}
        <Button
          variant="secondary"
          onClick={() => {
            onChoose(listing.path, listing.shown)
          }}
        >
          Switch here
        </Button>
      </div>
      {listing.entries.length === 0 ? (
        <p className="text-sm text-muted-foreground">No directories in it.</p>
      ) : (
        <ul aria-label={`Directories in ${listing.shown}`} className="flex flex-col">
          {listing.entries.map((entry) => (
            <li key={entry.path} className="flex items-baseline gap-item">
              <button
                type="button"
                aria-label={`Open ${entry.name}`}
                onClick={() => {
                  onOpen(entry.path)
                }}
                className="rounded-sm py-1 text-left font-mono text-primary underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              >
                {entry.name}
              </button>
              {entry.repository ? (
                <span className="text-sm text-muted-foreground">Repository</span>
              ) : null}
            </li>
          ))}
        </ul>
      )}
      {listing.truncated ? (
        <p className="text-sm text-muted-foreground">
          More than a thousand; type a path to reach the rest.
        </p>
      ) : null}
    </div>
  )
}
