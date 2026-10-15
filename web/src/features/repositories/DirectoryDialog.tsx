import type { UseQueryResult } from '@tanstack/react-query'
import { useRef, useState, type KeyboardEvent, type RefObject } from 'react'
import { flushSync } from 'react-dom'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { DirectoryListing, Repositories } from '@/api/generated/types.gen.ts'
import { Autocomplete, type AutocompleteGroup } from '@/lib/baseui/Autocomplete.tsx'
import { Dialog, DialogClose, DialogTitle } from '@/lib/baseui/Dialog.tsx'
import { Button } from '@/lib/Button.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { Failure, Reading } from '@/lib/Status.tsx'
import { ConfirmSwitch, type Destination } from './ConfirmSwitch.tsx'
import { Crumbs } from './Crumbs.tsx'
import { aboveTyped, joined, matching, splitTyped, trailOf, type Match } from './dirFilter.ts'
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
  const switching = useSwitching(switchTo)
  const field = useRef<HTMLInputElement>(null)
  const choose = (place: Destination) => {
    if (place.dir === repositories.here.dir) {
      setNotice(`You already work in ${place.shown}.`)
    } else {
      setAsked(place)
    }
  }
  const back = () => {
    // The list is drawn again at once, for its field to take back the focus
    // that Back, gone with the step, can no longer hold.
    flushSync(() => {
      setAsked(null)
    })
    field.current?.focus()
  }

  return (
    <Dialog
      initialFocus={asking === null ? field : false}
      onClose={() => {
        // A switch running holds the dialog open: its refusal is drawn in
        // the step, which closing would take away.
        if (!switching.running) {
          onClose()
        }
      }}
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
          switchTo={switching.switchTo}
          teller={teller}
          drawn="step"
          onCancel={back}
          onSwitched={onClose}
        />
      )}
    </Dialog>
  )
}

// useSwitching is switchTo, and whether a switch it was asked for runs now.
function useSwitching(switchTo: (dir: string) => Promise<Repositories>) {
  const [running, setRunning] = useState(false)

  return {
    running,
    switchTo: async (dir: string) => {
      setRunning(true)
      try {
        return await switchTo(dir)
      } finally {
        setRunning(false)
      }
    },
  }
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
// list stands at, and the switch to the row highlighted.
function Browse({ repositories, typed, field, notice, onType, onChoose }: BrowseProps) {
  const { parent, read, reading, listing, segment, matches } = useListing(typed)
  const [highlighted, setHighlighted] = useState<string>()
  const trail = listing === undefined ? undefined : trailOf(listing)
  const groups = listing === undefined ? [] : groupsOf(repositories, listing, typed, matches)
  const rows = groups.flatMap((group) => group.items)
  const chosen = rows.find((row) => keyOf(row) === highlighted) ?? rows[0]

  return (
    <>
      {trail === undefined ? null : (
        <Crumbs
          trail={trail}
          onGo={(above) => {
            onType(above)
            field.current?.focus()
          }}
        />
      )}
      <Autocomplete
        groups={groups}
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
        onHighlight={(row) => {
          setHighlighted(row === undefined ? undefined : keyOf(row))
        }}
        onChoose={(row) => {
          if (row.kind !== 'into') {
            onChoose(row.place)
          } else if (listing !== undefined) {
            onType(`${inside(listing, row.match.entry.name)}/`)
          }
        }}
        onKeyDown={(event: KeyboardEvent<HTMLInputElement>) => {
          if (!backspaceAtEnd(event) || !typed.endsWith('/')) {
            return
          }

          const up = trail === undefined ? aboveTyped(typed) : trail.above.at(-1)?.typed
          if (up !== undefined) {
            event.preventDefault()
            onType(up)
          }
        }}
        empty=""
      />
      <Said
        read={read}
        reading={reading}
        parent={parent}
        listing={listing}
        segment={segment}
        matches={matches.length}
        notice={notice}
      />
      <Footer
        place={chosen === undefined || listing === undefined ? undefined : placeOf(chosen, listing)}
        onChoose={onChoose}
      />
    </>
  )
}

interface SaidProps {
  // read is the read whose refusal is said; reading, the whole listing still
  // out.
  read: UseQueryResult<DirectoryListing>
  reading: boolean
  parent: string
  listing: DirectoryListing | undefined
  segment: string
  matches: number
  notice: string
}

// Said is what the dialog says under its list: a read refused, the directory
// being read, what the list stands at, and a notice. A listing narrowed by
// the name typed may answer, or be refused, while the whole one is still out;
// its rows or its refusal are shown, but the dialog says it is reading until
// the whole one answers, since that may hold more that match, and only then
// what the list stands at.
function Said({ read, reading, parent, listing, segment, matches, notice }: SaidProps) {
  return (
    <div className="flex flex-col gap-tight px-4 py-2 text-sm text-muted-foreground">
      {read.isError ? (
        <Failure key={read.errorUpdateCount}>
          {apiErrorMessage(read.error, 'That directory could not be listed.')}
        </Failure>
      ) : null}
      {reading ? (
        <Reading>Reading {parent === '' ? 'where the server works' : parent}…</Reading>
      ) : (
        listing !== undefined && <Standing listing={listing} segment={segment} matches={matches} />
      )}
      {notice === '' ? null : <p role="status">{notice}</p>}
    </div>
  )
}

// backspaceAtEnd reports a Backspace that deletes one letter at the end of
// what was typed: none selected, and no word taken with a modifier.
function backspaceAtEnd(event: KeyboardEvent<HTMLInputElement>): boolean {
  const { selectionStart, selectionEnd, value } = event.currentTarget
  const modified = event.ctrlKey || event.metaKey || event.altKey

  return (
    event.key === 'Backspace' &&
    !modified &&
    selectionStart === selectionEnd &&
    selectionEnd === value.length
  )
}

// useListing reads the directory the field names, and again for the names
// starting with what was typed after its last slash when the first listing
// was cut short, or when that name starts with a dot, since a hidden
// directory is listed only then: the directory read, the read whose refusal
// is shown, whether the whole listing is still out, what the two list
// together, the name typed, and the directories that hold it.
function useListing(typed: string) {
  const { parent, segment } = splitTyped(typed)
  const all = useDirectories(parent)
  const narrowing =
    segment !== '' && (all.data?.truncated_to !== undefined || segment.startsWith('.'))
  const narrowed = useDirectories(parent, segment, narrowing)
  const listing = narrowing ? joined(all.data, narrowed.data) : all.data

  return {
    parent,
    read: narrowing && narrowed.isError ? narrowed : all,
    reading: all.isPending,
    listing,
    segment,
    matches: listing === undefined ? [] : matching(listing.entries, segment),
  }
}

// Footer is the dialog's way out, and the switch to the place the row
// highlighted names once there is one.
function Footer({
  place,
  onChoose,
}: {
  place: Destination | undefined
  onChoose: (place: Destination) => void
}) {
  return (
    <div className="flex flex-wrap justify-end gap-item border-t border-border px-4 py-3">
      <DialogClose render={<Button variant="secondary">Cancel</Button>} />
      {place === undefined ? null : (
        <Button
          variant="primary"
          onClick={() => {
            onChoose(place)
          }}
        >
          Switch to {place.shown}
        </Button>
      )}
    </div>
  )
}

// placeOf is where a row switches to: the directory it goes into, or the
// place it names.
function placeOf(row: Row, listing: DirectoryListing): Destination {
  return row.kind === 'into'
    ? { dir: row.match.entry.path, shown: inside(listing, row.match.entry.name) }
    : row.place
}

// inside is a directory in the one listed, as the field writes it.
function inside(listing: DirectoryListing, name: string): string {
  return `${listing.shown === '/' ? '' : listing.shown}/${name}`
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
    listing.truncated_to === undefined
      ? undefined
      : `First ${String(listing.truncated_to)} shown; keep typing to narrow.`,
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
