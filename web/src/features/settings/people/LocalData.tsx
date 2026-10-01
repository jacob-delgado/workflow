import { type RefObject, useEffect, useId, useRef, useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { LocalData as Listing, LocalDataFile } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, type Teller, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { type CleanScope, useCleanLocalData, useLocalData } from './localDataApi.ts'

// LocalData is Settings' Local data area: where workflow keeps what it learns
// between sessions, each database file with its size and what it holds, and
// the two cleans `workflow db-clean` makes, each behind a confirm step.
export function LocalData() {
  const headingId = useId()

  return (
    <section aria-labelledby={headingId} className="flex max-w-2xl flex-col gap-group">
      <h2 id={headingId} className="text-base font-semibold">
        Local data
      </h2>
      <p className="text-xs text-muted-foreground">
        workflow keeps a cache a session makes again, and the people and group associations you
        decided, in two files on this machine.
      </p>
      <LocalDataBody />
    </section>
  )
}

// LocalDataBody is the listing and its cleans, or why it could not be read.
function LocalDataBody() {
  const query = useLocalData()

  if (query.isPending) {
    return <p className="text-sm text-muted-foreground">Reading the local data…</p>
  }

  if (query.isError) {
    return (
      <div className="flex flex-col items-start gap-item">
        <p className="text-sm text-destructive">
          {apiErrorMessage(query.error, 'The local data could not be read.')}
        </p>
        <Button
          variant="secondary"
          disabled={query.isFetching}
          onClick={() => {
            void query.refetch()
          }}
        >
          Read the local data again
        </Button>
      </div>
    )
  }

  return (
    <>
      <Files listing={query.data} />
      <Cleans files={query.data.files} />
    </>
  )
}

// Files is the store's directory and a row per database file in it.
function Files({ listing }: { listing: Listing }) {
  return (
    <div className="flex flex-col gap-item text-sm">
      <p>
        <span className="text-muted-foreground">Kept in </span>
        <code className="font-mono break-all">{listing.dir}</code>
      </p>
      {listing.files.length === 0 ? (
        <p className="text-muted-foreground">No local data: there is nothing to clean.</p>
      ) : (
        <table aria-label="Local databases" className="w-full table-fixed text-left">
          <thead className="text-xs text-muted-foreground">
            <tr>
              <th scope="col" className="w-28 py-1 font-medium">
                File
              </th>
              <th scope="col" className="w-16 py-1 font-medium">
                Kind
              </th>
              <th scope="col" className="w-20 py-1 font-medium">
                Size
              </th>
              <th scope="col" className="py-1 font-medium">
                Holds
              </th>
            </tr>
          </thead>
          <tbody>
            {listing.files.map((file) => (
              <tr key={file.name} className="border-t border-border align-top">
                <td className="py-1 font-mono break-all">{file.name}</td>
                <td className="py-1">{kindNames[file.kind]}</td>
                <td className="py-1 tabular-nums">{humanBytes(file.bytes)}</td>
                <td className="py-1 break-words">{holdings(file)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// kindNames are how each kind of file reads in the table.
const kindNames: Record<LocalDataFile['kind'], string> = { cache: 'Cache', kept: 'Kept' }

// humanBytes is a size in bytes, KiB or MiB, with one decimal past bytes, as
// `workflow db-clean` prints it.
function humanBytes(bytes: number): string {
  const unit = 1024
  if (bytes < unit) {
    return `${String(bytes)} B`
  }

  return bytes < unit * unit
    ? `${(bytes / unit).toFixed(1)} KiB`
    : `${(bytes / (unit * unit)).toFixed(1)} MiB`
}

// holdings says what a file holds, or that it could not be read as one.
function holdings(file: LocalDataFile): string {
  if (file.holds.length === 0) {
    return 'Not readable as a database'
  }

  return file.holds.map((held) => `${held.what}: ${String(held.count)}`).join(', ')
}

// Cleans are the two cleans, each opening its confirm step, or under
// --dry-run the sentence saying they are held back; and what the last one
// said.
function Cleans({ files }: { files: LocalDataFile[] }) {
  const dryRun = useHealthStore((state) => state.health?.dry_run === true)
  const outcome = useOutcome()

  if (files.length === 0 && outcome.said === null) {
    return null
  }

  return (
    <div className="flex flex-col items-start gap-item">
      {dryRun ? (
        <p className="text-sm text-muted-foreground">
          Cleaning is held back: workflow was started with{' '}
          <code className="font-mono">--dry-run</code>, so nothing is removed.
        </p>
      ) : (
        <CleanSteps files={files} tell={outcome} />
      )}
      <OutcomeLine said={outcome.said} />
    </div>
  )
}

// CleanSteps are the two openers, or the confirm step one of them opened,
// and the reason the last clean was refused.
function CleanSteps({ files, tell }: { files: LocalDataFile[]; tell: Teller }) {
  const [confirming, setConfirming] = useState<CleanScope | null>(null)
  const [refusedFrom, setRefusedFrom] = useState<CleanScope>('cache')
  const cacheOpener = useRef<HTMLButtonElement>(null)
  const allOpener = useRef<HTMLButtonElement>(null)
  const handBackTo = useRef<CleanScope | null>(null)
  const cleanLocalData = useCleanLocalData()
  const clean = useAsyncAction(cleanLocalData, {
    fallback: 'The local data was not cleaned. Try again.',
    done: (_, scope) => `Removed ${namesReached(files, scope)}.`,
    onStart: tell.clear,
    onDone: tell.say,
  })

  // Focus goes back to the opener a Cancel closed, or to the one whose clean
  // was refused when focus had fallen to the page.
  useEffect(() => {
    const opener = { cache: cacheOpener, all: allOpener, none: null }[handBackTo.current ?? 'none']
    if (opener?.current) {
      handBackTo.current = null
      opener.current.focus()
    }
  })
  useEffect(() => {
    if (clean.state === 'error' && document.activeElement === document.body) {
      ;(refusedFrom === 'cache' ? cacheOpener : allOpener).current?.focus()
    }
  }, [clean.state, refusedFrom])

  if (confirming !== null) {
    return (
      <CleanConfirm
        scope={confirming}
        names={namesReached(files, confirming)}
        onCancel={() => {
          handBackTo.current = confirming
          setConfirming(null)
        }}
        onClean={() => {
          setRefusedFrom(confirming)
          setConfirming(null)
          void clean.run(confirming)
        }}
      />
    )
  }

  return (
    <>
      <CleanOpeners
        files={files}
        running={clean.state === 'running'}
        cacheOpener={cacheOpener}
        allOpener={allOpener}
        onOpen={setConfirming}
      />
      {clean.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {clean.error}
        </p>
      ) : null}
    </>
  )
}

interface CleanOpenersProps {
  files: LocalDataFile[]
  running: boolean
  cacheOpener: RefObject<HTMLButtonElement | null>
  allOpener: RefObject<HTMLButtonElement | null>
  onOpen: (scope: CleanScope) => void
}

// CleanOpeners are the buttons that open each clean's confirm step: the cache
// while there is one, and everything while there is anything.
function CleanOpeners({ files, running, cacheOpener, allOpener, onOpen }: CleanOpenersProps) {
  return (
    <div className="flex flex-wrap items-center gap-item">
      {files.some((file) => file.kind === 'cache') ? (
        <Button
          variant="secondary"
          ref={cacheOpener}
          disabled={running}
          onClick={() => {
            onOpen('cache')
          }}
        >
          Clean cache…
        </Button>
      ) : null}
      {files.length > 0 ? (
        <Button
          variant="secondary"
          ref={allOpener}
          disabled={running}
          onClick={() => {
            onOpen('all')
          }}
        >
          Clean everything…
        </Button>
      ) : null}
      {running ? <span className="text-sm text-muted-foreground">Cleaning…</span> : null}
    </div>
  )
}

// namesReached names the files a clean of scope removes, joined by "and".
function namesReached(files: LocalDataFile[], scope: CleanScope): string {
  return files
    .filter((file) => file.kind === 'cache' || scope === 'all')
    .map((file) => file.name)
    .join(' and ')
}

interface CleanConfirmProps {
  scope: CleanScope
  names: string
  onCancel: () => void
  onClean: () => void
}

// CleanConfirm asks before a clean, and takes focus as it opens, so a screen
// reader hears the question. Cleaning everything says what is lost with the
// kept file.
function CleanConfirm({ scope, names, onCancel, onClean }: CleanConfirmProps) {
  const question = useFocusOnMount<HTMLDivElement>()
  const questionId = useId()

  return (
    <div
      ref={question}
      role="group"
      aria-labelledby={questionId}
      tabIndex={-1}
      className="flex flex-col items-start gap-item text-sm"
    >
      <p id={questionId}>Remove {names}?</p>
      <p className="text-muted-foreground">
        {scope === 'all'
          ? 'Whom each code owner is on Slack and each repository’s groups go with it: people and group associations will be asked again.'
          : 'The last scope, what was announced and the cached issue lists are made again as you work.'}
      </p>
      <div className="flex items-center gap-item">
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="primary" onClick={onClean}>
          Clean
        </Button>
      </div>
    </div>
  )
}
