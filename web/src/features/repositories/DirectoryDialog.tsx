import { useRef, useState, type KeyboardEvent, type RefObject } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { DirectoryListing, Repositories } from '@/api/generated/types.gen.ts'
import { Autocomplete, type AutocompleteGroup } from '@/lib/baseui/Autocomplete.tsx'
import { Dialog, DialogClose, DialogTitle } from '@/lib/baseui/Dialog.tsx'
import { Button } from '@/lib/Button.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { Failure, Reading } from '@/lib/Status.tsx'
import { cn } from '@/lib/utils.ts'
import { ConfirmSwitch, type Destination } from './ConfirmSwitch.tsx'
import { matching, splitTyped, upFrom, type Match } from './dirFilter.ts'
import { useDirectories } from './repositoriesApi.ts'

// Trade-off TRADE-49: the list draws at most this many matches, and says how
// many more there are, rather than drawing thousands of rows or windowing them.
const rowCap = 200

// Row is one thing the list offers: a directory to go into, or a place to
// switch to — the directory listed, a worktree, a favorite.
type Row =
  | { kind: 'into'; match: Match }
  | { kind: 'here' | 'worktree' | 'favorite'; place: Destination; repository: boolean }

interface DirectoryDialogProps {
  repositories: Repositories
  // asking is the switch the dialog opens on, asked for elsewhere; null opens
  // it on the list.
  asking: Destination | null
  switchTo: (dir: string) => Promise<Repositories>
  teller: Teller
  onClose: () => void
}

// DirectoryDialog goes to another directory: a field naming a directory —
// from where the server works, from your home after ~, or from the root —
// over the directories in it, narrowed by the name typed after its last
// slash, and the worktrees and favorites while no path is typed. Enter on a
// directory goes into it, Backspace after a slash goes up, and a place chosen
// is asked once more in the dialog before the switch is made.
export function DirectoryDialog({
  repositories,
  asking,
  switchTo,
  teller,
  onClose,
}: DirectoryDialogProps) {
  const [typed, setTyped] = useState('')
  const [asked, setAsked] = useState(asking)
  const [notice, setNotice] = useState('')
  const field = useRef<HTMLInputElement>(null)
  const choose = (place: Destination) => {
    if (place.dir === repositories.here.dir) {
      setNotice(`You already work in ${place.shown}.`)
    } else {
      setAsked(place)
    }
  }

  return (
    <Dialog
      initialFocus={asking === null ? field : false}
      onClose={onClose}
      className="mt-[10dvh] flex max-h-[80dvh] w-[40rem] flex-col self-start overflow-hidden shadow-lg"
    >
      <DialogTitle className="border-b border-border px-4 py-3 text-base font-semibold">
        Go to a directory
      </DialogTitle>
      {asked === null ? (
        <Browse
          repositories={repositories}
          typed={typed}
          field={field}
          notice={notice}
          onType={(next) => {
            setTyped(next)
            setNotice('')
          }}
          onChoose={choose}
        />
      ) : (
        <ConfirmSwitch
          destination={asked}
          switchTo={switchTo}
          teller={teller}
          drawn="step"
          onCancel={() => {
            setAsked(null)
          }}
          onSwitched={onClose}
        />
      )}
    </Dialog>
  )
}

interface BrowseProps {
  repositories: Repositories
  typed: string
  field: RefObject<HTMLInputElement | null>
  notice: string
  onType: (typed: string) => void
  onChoose: (place: Destination) => void
}

// Browse is the dialog's list: the field, the directories it names, what the
// list stands at, and the switch to the directory listed.
function Browse({ repositories, typed, field, notice, onType, onChoose }: BrowseProps) {
  const { read, listing, segment, matches } = useListing(typed)

  return (
    <>
      {listing === undefined ? null : <Crumbs listing={listing} onGo={onType} />}
      <Autocomplete
        groups={listing === undefined ? [] : groupsOf(repositories, listing, typed, matches)}
        typed={typed}
        onType={onType}
        label="Directory"
        listLabel="Directories"
        placeholder="A name here, or a path from ~ or /"
        inputRef={field}
        mono
        itemKey={keyOf}
        itemText={(row) => (row.kind === 'into' ? row.match.entry.name : row.place.shown)}
        renderItem={(row) => <RowWords row={row} />}
        onChoose={(row) => {
          if (row.kind !== 'into') {
            onChoose(row.place)
          } else if (listing !== undefined) {
            onType(`${listing.shown === '/' ? '' : listing.shown}/${row.match.entry.name}/`)
          }
        }}
        onKeyDown={(event: KeyboardEvent<HTMLInputElement>) => {
          if (event.key === 'Backspace' && typed.endsWith('/') && upFrom(typed) !== typed) {
            event.preventDefault()
            onType(upFrom(typed))
          }
        }}
        empty=""
      />
      <div className="flex flex-col gap-tight px-4 py-2 text-sm text-muted-foreground">
        {read.isError ? (
          <Failure key={read.errorUpdateCount}>
            {apiErrorMessage(read.error, 'That directory could not be listed.')}
          </Failure>
        ) : null}
        {listing === undefined ? (
          read.isPending && <Reading>Reading…</Reading>
        ) : (
          <Standing listing={listing} segment={segment} matches={matches.length} />
        )}
        {notice === '' ? null : <p role="status">{notice}</p>}
      </div>
      <Footer listing={listing} onChoose={onChoose} />
    </>
  )
}

// useListing reads the directory the field names, and again for the names
// starting with what was typed after its last slash when the first listing
// was cut short: the read that answers, its listing, the name typed, and the
// directories in it that hold that name.
function useListing(typed: string) {
  const { parent, segment } = splitTyped(typed)
  const all = useDirectories(parent)
  const narrowed = useDirectories(parent, segment, all.data?.truncated === true && segment !== '')
  const read = narrowed.data === undefined ? all : narrowed
  const listing = read.data

  return {
    read,
    listing,
    segment,
    matches: listing === undefined ? [] : matching(listing.entries, segment),
  }
}

// Footer is the dialog's way out, and the switch to the directory listed once
// one is.
function Footer({
  listing,
  onChoose,
}: {
  listing: DirectoryListing | undefined
  onChoose: (place: Destination) => void
}) {
  return (
    <div className="flex flex-wrap justify-end gap-item border-t border-border px-4 py-3">
      <DialogClose render={<Button variant="secondary">Cancel</Button>} />
      {listing === undefined ? null : (
        <Button
          variant="primary"
          onClick={() => {
            onChoose({ dir: listing.path, shown: listing.shown })
          }}
        >
          Switch to {listing.shown}
        </Button>
      )}
    </div>
  )
}

// groupsOf is what the list offers: while no path is typed, the worktrees and
// favorites to switch to, then the directory listed; and in it, switch here
// before its directories while no name is typed, and after them once one is,
// so the first match is the one Enter takes.
function groupsOf(
  repositories: Repositories,
  listing: DirectoryListing,
  typed: string,
  matches: Match[],
): AutocompleteGroup<Row>[] {
  const here: Row = {
    kind: 'here',
    place: { dir: listing.path, shown: listing.shown },
    repository: false,
  }
  const into: Row[] = matches.slice(0, rowCap).map((match) => ({ kind: 'into', match }))
  const inHere = {
    name: `In ${listing.shown}`,
    items: splitTyped(typed).segment === '' ? [here, ...into] : [...into, here],
  }
  if (typed.includes('/') || typed.startsWith('~')) {
    return [inHere]
  }

  return [...placesOf(repositories, typed), inHere]
}

// placesOf is the worktrees and favorites there are to switch to whose
// directory's name holds what was typed, each under its heading.
function placesOf(repositories: Repositories, typed: string): AutocompleteGroup<Row>[] {
  const named = (shown: string) => {
    const name = shown.slice(shown.lastIndexOf('/') + 1)

    return matching([{ name, path: shown, repository: false }], typed).length > 0
  }
  const worktrees: Row[] = repositories.worktrees
    .filter((worktree) => worktree.state === 'worktree' && named(worktree.shown))
    .map((worktree) => ({ kind: 'worktree', place: worktree, repository: false }))
  const favorites: Row[] = repositories.favorites
    .filter((favorite) => favorite.state !== 'missing' && favorite.state !== 'here')
    .filter((favorite) => named(favorite.shown))
    .map((favorite) => ({
      kind: 'favorite',
      place: favorite,
      repository: favorite.state === 'repository',
    }))

  return [
    { name: 'Worktrees', items: worktrees },
    { name: 'Favorites', items: favorites },
  ].filter((group) => group.items.length > 0)
}

// keyOf names a row apart from every other the list draws.
function keyOf(row: Row): string {
  return row.kind === 'into' ? `into ${row.match.entry.path}` : `${row.kind} ${row.place.dir}`
}

// RowWords is a row as the list draws it: a directory's name with the letters
// typed in bold, or the place a switch goes to, and whether it is a
// repository or a worktree.
function RowWords({ row }: { row: Row }) {
  if (row.kind === 'into') {
    return (
      <>
        <span className="min-w-0 truncate font-mono">
          <Lettered name={row.match.entry.name} bold={row.match.letters} />
        </span>
        {row.match.entry.repository ? <Kind>repository</Kind> : null}
      </>
    )
  }

  return (
    <>
      <span className="min-w-0 truncate">
        {row.kind === 'here' ? (
          <span className="mr-2 text-muted-foreground">switch here</span>
        ) : null}
        {row.kind === 'favorite' ? <span aria-hidden>★ </span> : null}
        <span className="font-mono">{row.place.shown}</span>
      </span>
      {row.kind === 'worktree' ? <Kind>worktree</Kind> : null}
      {row.repository ? <Kind>repository</Kind> : null}
    </>
  )
}

// Kind says what a row's directory is, in git's hue.
function Kind({ children }: { children: string }) {
  return <span className="shrink-0 text-xs text-git">{children}</span>
}

// Lettered is a name with the letters at bold set in bold.
function Lettered({ name, bold }: { name: string; bold: number[] }) {
  return Array.from(name).map((letter, place) =>
    bold.includes(place) ? (
      // eslint-disable-next-line react/no-array-index-key -- a letter's place is what tells it apart, and the name does not change under it
      <b key={place} className="font-bold text-foreground">
        {letter}
      </b>
    ) : (
      letter
    ),
  )
}

// Crumbs are the directories above the one listed, each a button that lists
// it: the way up past where a typed path can go.
function Crumbs({ listing, onGo }: { listing: DirectoryListing; onGo: (typed: string) => void }) {
  const parts = listing.shown.split('/')
  const above = parts.slice(0, -1)

  if (above.length === 0) {
    return null
  }

  return (
    <nav aria-label="Up from here" className="flex flex-wrap items-center gap-1 px-4 pt-2 text-sm">
      {above.map((part, index) => {
        const path = `${parts.slice(0, index + 1).join('/')}/`

        return (
          <span key={path} className="flex items-center gap-1">
            <button
              type="button"
              aria-label={`Up to ${path === '/' ? '/' : path.slice(0, -1)}`}
              onClick={() => {
                onGo(path)
              }}
              className={cn(
                'rounded-sm font-mono text-primary underline-offset-2 hover:underline',
                'focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
              )}
            >
              {part === '' ? '/' : part}
            </button>
            <span aria-hidden className="text-muted-foreground">
              /
            </span>
          </span>
        )
      })}
      <span className="font-mono text-muted-foreground">{parts.at(-1)}</span>
    </nav>
  )
}

interface StandingProps {
  listing: DirectoryListing
  segment: string
  matches: number
}

// Standing says what the list stands at: how many of the directory's match,
// that none do or that it holds none, that it was cut short, and how many
// more match than are drawn.
function Standing({ listing, segment, matches }: StandingProps) {
  const lines = [
    countLine(listing, segment, matches),
    listing.truncated ? 'First 1000 shown; keep typing to narrow.' : undefined,
    matches > rowCap ? `${String(matches - rowCap)} more match; keep typing to narrow.` : undefined,
  ].filter((line) => line !== undefined)

  return (
    <div aria-live="polite" className="flex flex-col gap-tight">
      {lines.map((line) => (
        <p key={line}>{line}</p>
      ))}
    </div>
  )
}

// countLine is how many directories the one listed holds, or how many of them
// hold the name typed.
function countLine(listing: DirectoryListing, segment: string, matches: number): string {
  const total = listing.entries.length
  if (total === 0) {
    return `No directories in ${listing.shown}.`
  }

  if (segment === '') {
    return `${String(total)} ${total === 1 ? 'directory' : 'directories'} in ${listing.shown}`
  }

  return matches === 0
    ? `Nothing in ${listing.shown} matches ${segment}.`
    : `${String(matches)} of ${String(total)} in ${listing.shown}`
}
