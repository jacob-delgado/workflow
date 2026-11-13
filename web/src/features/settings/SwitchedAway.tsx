import { useState } from 'react'
import type { UseQueryResult } from '@tanstack/react-query'
import type { Repositories } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useRepositories } from '@/features/repositories/repositoriesApi.ts'
import { useFocusOnMount } from '@/lib/focus.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import type { ConfigRead } from './configApi.ts'

// Awaited is a switch the stream reported while Settings was open, until the
// configuration and where the server works, read again after it, answer: the
// directory left, the revision of the file the form was showing and the files
// that applied there, and when each read last answered before.
interface Awaited {
  left: string
  revision: string | undefined
  files: string[] | undefined
  answered: number
  placed: number
}

// Followed is where Settings stands over the switches the stream reports:
// where the server works, the switch awaiting its read, the form's generation,
// which a read of another file ends, and the directory whose unsaved edits
// that read dropped, or null.
interface Followed {
  here: string | undefined
  awaited: Awaited | null
  form: number
  left: string | null
}

// useReadAfterSwitch follows a switch made in another tab or the terminal's
// interface while Settings is open, whose configuration is read again in
// place. A read of the same revision, where the same files apply, keeps the
// form and its edits: a revision is a digest of the files' bytes, not of their
// paths, so a copy of the file reads the same, and the files where the server
// works tell the two apart. A read of another revision, or of other files, or
// either read refused, ends the form, and says so when it held unsaved edits,
// which were for the directory left. form keys the form, and left is the
// directory whose edits were dropped.
export function useReadAfterSwitch(query: UseQueryResult<ConfigRead>) {
  const here = useSnapshotStore((state) => state.snapshot?.here)
  const unsaved = useUiStore((state) => state.unsaved)
  const place = useRepositories()
  const [followed, setFollowed] = useState<Followed>({ here, awaited: null, form: 0, left: null })

  // Set while rendering, as React has state follow what it reads, so the frame
  // that draws the read after a switch is the one that keeps or ends the form.
  const read = { query, place }
  const next =
    here === followed.here ? afterRead(followed, read, unsaved) : switched(followed, here, read)
  if (next !== followed) {
    setFollowed(next)
  }

  return { form: next.form, left: next.left }
}

// Reads are the reads a switch is followed by: the configuration, and where
// the server works, with the files that apply there.
interface Reads {
  query: UseQueryResult<ConfigRead>
  place: UseQueryResult<Repositories>
}

// answeredAt is when the read last answered, with what it read or a refusal.
function answeredAt(query: UseQueryResult): number {
  return Math.max(query.dataUpdatedAt, query.errorUpdatedAt)
}

// filesOf are the configuration files that apply where the server works, as
// its read last answered them, or undefined when none has: a switch lets go of
// what was read before it, so a read refused after one leaves none.
function filesOf(place: UseQueryResult<Repositories>): string[] | undefined {
  return place.data?.here.config
}

// sameFiles says whether the files that applied before a switch, known, are
// the files that apply after it.
function sameFiles(before: string[] | undefined, after: string[] | undefined): boolean {
  return before !== undefined && JSON.stringify(before) === JSON.stringify(after)
}

// switched is where Settings stands once the server works in here: awaiting
// the reads again of the configuration and of where the server works, unless
// it worked nowhere before, or works nowhere now.
function switched(followed: Followed, here: string | undefined, read: Reads): Followed {
  const left = followed.here
  if (left === undefined || here === undefined) {
    return { ...followed, here, awaited: null }
  }

  const { query, place } = read
  const awaited = {
    left,
    revision: query.data?.revision,
    files: filesOf(place),
    answered: answeredAt(query),
    placed: answeredAt(place),
  }

  return { ...followed, here, awaited }
}

// afterRead is where Settings stands once the reads awaited settle whether the
// form is over the same file: the same form, or a new one over another file or
// a refusal, saying so when the form held unsaved edits. Until then, nothing
// changes.
function afterRead(followed: Followed, read: Reads, unsaved: boolean): Followed {
  const { awaited } = followed
  if (awaited === null) {
    return followed
  }

  const verdict = verdictOn(awaited, read)
  if (verdict === 'awaiting') {
    return followed
  }

  const moved = verdict === 'moved'

  return {
    ...followed,
    awaited: null,
    form: moved ? followed.form + 1 : followed.form,
    left: moved && unsaved ? awaited.left : null,
  }
}

// Verdict is what the reads after a switch say of the file the form was over:
// still awaiting them, the same file, or another file or a refusal.
type Verdict = 'awaiting' | 'same' | 'moved'

// verdictOn is what the reads after the switch awaited say. The first answer
// after the switch is the read it sent, which cancels any read still out when
// it went. A configuration of another revision, or refused, has moved at once,
// while the form over it still stands; one of the same revision waits for
// where the server works, whose files tell the same file from a copy.
function verdictOn(awaited: Awaited, { query, place }: Reads): Verdict {
  if (answeredAt(query) <= awaited.answered) {
    return 'awaiting'
  }
  if (query.isError || query.data?.revision !== awaited.revision) {
    return 'moved'
  }
  if (answeredAt(place) <= awaited.placed) {
    return 'awaiting'
  }

  return sameFiles(awaited.files, filesOf(place)) ? 'same' : 'moved'
}

// EditsLeft says the form's unsaved edits were for the directory a switch made
// elsewhere left, and were dropped with its file. It takes the focus, which
// went with the form it was in.
export function EditsLeft({ left }: { left: string | null }) {
  return left === null ? null : <EditsLeftSaid key={left} left={left} />
}

function EditsLeftSaid({ left }: { left: string }) {
  const said = useFocusOnMount<HTMLParagraphElement>()

  return (
    <p
      ref={said}
      role="status"
      tabIndex={-1}
      className="rounded-md border border-warning/40 bg-warning-surface px-3 py-2 text-sm"
    >
      Your unsaved changes were for {left}, which the server has left, so they were dropped.
    </p>
  )
}
