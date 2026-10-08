import { type RefObject, useEffect, useId, useRef, useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type {
  LocalData as Listing,
  LocalDataConsequences,
  LocalDataFile,
} from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { Button } from '@/lib/Button.tsx'
import { Reading, Unread } from '@/lib/Status.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, type Teller, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { type RemoveScope, useRemoveLocalData, useLocalData } from './localDataApi.ts'
import { useHoldShortcuts } from '@/features/keyboard/useShortcut.ts'

// LocalData is Settings' Local data area: where workflow keeps what it learns
// between sessions, each database file with its size and what it holds, and
// the two removals `workflow db-clean` makes, each behind a confirm step.
export function LocalData() {
  const headingId = useId()

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-group">
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

// LocalDataBody is the listing and its removals, or why it could not be read.
function LocalDataBody() {
  const query = useLocalData()

  if (query.isPending) {
    return <Reading>Reading the local data…</Reading>
  }

  if (query.isError) {
    return (
      <Unread
        reason={apiErrorMessage(query.error, 'The local data could not be read.')}
        refusals={query.errorUpdateCount}
        retrying={query.isFetching}
        onRetry={() => {
          void query.refetch()
        }}
      />
    )
  }

  return (
    <>
      <Files listing={query.data} />
      <Removals files={query.data.files} consequences={query.data.consequences} />
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
        <p className="text-muted-foreground">No local data: there is nothing to remove.</p>
      ) : (
        <table aria-label="Local data files" className="w-full table-fixed text-left">
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
                <td className="py-1 tabular-nums">{file.size}</td>
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

// holdings says what a file holds, or that it could not be read as one.
function holdings(file: LocalDataFile): string {
  if (file.holds.length === 0) {
    return 'Not readable as a database'
  }

  return file.holds.map((held) => `${held.what}: ${String(held.count)}`).join(', ')
}

// Removals are the two removals, each opening its confirm step, or under
// --dry-run the sentence saying they are held back; and what the last one
// said.
function Removals({
  files,
  consequences,
}: {
  files: LocalDataFile[]
  consequences: LocalDataConsequences
}) {
  const dryRun = useHealthStore((state) => state.health?.dry_run === true)
  const outcome = useOutcome()

  if (files.length === 0 && outcome.said === null) {
    return null
  }

  return (
    <div className="flex flex-col items-start gap-item">
      {dryRun ? (
        <p className="text-sm text-muted-foreground">
          Removing is held back: workflow was started with{' '}
          <code className="font-mono">--dry-run</code>, so nothing is removed.
        </p>
      ) : (
        <RemoveSteps files={files} consequences={consequences} tell={outcome} />
      )}
      <OutcomeLine said={outcome.said} />
    </div>
  )
}

// RemoveStepsProps are the files a removal reaches, what the server says each
// removal takes with it, and where its outcome is told.
interface RemoveStepsProps {
  files: LocalDataFile[]
  consequences: LocalDataConsequences
  tell: Teller
}

// RemoveSteps are the two openers, or the confirm step one of them opened,
// and the reason the last removal was refused.
function RemoveSteps({ files, consequences, tell }: RemoveStepsProps) {
  const [confirming, setConfirming] = useState<RemoveScope | null>(null)
  const [refusedFrom, setRefusedFrom] = useState<RemoveScope>('cache')
  const cacheOpener = useRef<HTMLButtonElement>(null)
  const allOpener = useRef<HTMLButtonElement>(null)
  const handBackTo = useRef<RemoveScope | null>(null)
  const removeLocalData = useRemoveLocalData()
  const removal = useAsyncAction(removeLocalData, {
    fallback: 'The local data was not removed. Try again.',
    done: (_, scope) => `Removed ${namesReached(files, scope)}.`,
    onStart: tell.clear,
    onDone: tell.say,
  })

  // Focus goes back to the opener a Cancel closed, or to the one whose removal
  // was refused when focus had fallen to the page.
  useEffect(() => {
    const opener = { cache: cacheOpener, all: allOpener, none: null }[handBackTo.current ?? 'none']
    if (opener?.current) {
      handBackTo.current = null
      opener.current.focus()
    }
  })
  useEffect(() => {
    if (removal.state === 'error' && document.activeElement === document.body) {
      ;(refusedFrom === 'cache' ? cacheOpener : allOpener).current?.focus()
    }
  }, [removal.state, refusedFrom])

  if (confirming !== null) {
    return (
      <RemoveConfirm
        names={namesReached(files, confirming)}
        consequence={consequences[confirming]}
        onCancel={() => {
          handBackTo.current = confirming
          setConfirming(null)
        }}
        onRemove={() => {
          setRefusedFrom(confirming)
          setConfirming(null)
          void removal.run(confirming)
        }}
      />
    )
  }

  return (
    <>
      <RemoveOpeners
        files={files}
        running={removal.state === 'running'}
        cacheOpener={cacheOpener}
        allOpener={allOpener}
        onOpen={setConfirming}
      />
      {removal.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {removal.error}
        </p>
      ) : null}
    </>
  )
}

interface RemoveOpenersProps {
  files: LocalDataFile[]
  running: boolean
  cacheOpener: RefObject<HTMLButtonElement | null>
  allOpener: RefObject<HTMLButtonElement | null>
  onOpen: (scope: RemoveScope) => void
}

// RemoveOpeners are the buttons that open each removal's confirm step: the cache
// while there is one, and everything while there is anything.
function RemoveOpeners({ files, running, cacheOpener, allOpener, onOpen }: RemoveOpenersProps) {
  return (
    <div className="flex flex-wrap items-center gap-item">
      {files.some((file) => file.kind === 'cache') ? (
        <Button
          variant="secondary"
          ref={cacheOpener}
          held={running}
          onClick={() => {
            onOpen('cache')
          }}
        >
          Remove cache…
        </Button>
      ) : null}
      {files.length > 0 ? (
        <Button
          variant="secondary"
          ref={allOpener}
          held={running}
          onClick={() => {
            onOpen('all')
          }}
        >
          Remove everything…
        </Button>
      ) : null}
      {running ? <span className="text-sm text-muted-foreground">Removing…</span> : null}
    </div>
  )
}

// namesReached names the files a removal of scope removes, joined by "and".
function namesReached(files: LocalDataFile[], scope: RemoveScope): string {
  return files
    .filter((file) => file.kind === 'cache' || scope === 'all')
    .map((file) => file.name)
    .join(' and ')
}

interface RemoveConfirmProps {
  names: string
  consequence: string
  onCancel: () => void
  onRemove: () => void
}

// RemoveConfirm asks before a removal, and takes focus as it opens, so a screen
// reader hears the question. Removing everything says what is lost with the
// kept file.
function RemoveConfirm({ names, consequence, onCancel, onRemove }: RemoveConfirmProps) {
  const question = useFocusOnMount<HTMLDivElement>()
  useHoldShortcuts()
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
      <p className="text-muted-foreground">{consequence}</p>
      <div className="flex items-center gap-item">
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="primary" onClick={onRemove}>
          Remove
        </Button>
      </div>
    </div>
  )
}
