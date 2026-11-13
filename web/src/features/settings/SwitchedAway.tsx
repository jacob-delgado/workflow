import { useState } from 'react'
import type { UseQueryResult } from '@tanstack/react-query'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useFocusOnMount } from '@/lib/focus.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import type { ConfigRead } from './configApi.ts'

// Awaited is a switch the stream reported while Settings was open, until the
// configuration read again after it answers: the directory left, the revision
// of the file the form was showing, and when the read last answered before.
interface Awaited {
  left: string
  revision: string | undefined
  answered: number
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
// place. A read of the same revision, the same file, keeps the form and its
// edits; one of another file, or a read refused, ends the form, and says so
// when it held unsaved edits, which were for the directory left. form keys the
// form, and left is the directory whose edits were dropped.
export function useReadAfterSwitch(query: UseQueryResult<ConfigRead>) {
  const here = useSnapshotStore((state) => state.snapshot?.here)
  const unsaved = useUiStore((state) => state.unsaved)
  const [followed, setFollowed] = useState<Followed>({ here, awaited: null, form: 0, left: null })

  // Set while rendering, as React has state follow what it reads, so the frame
  // that draws the read after a switch is the one that keeps or ends the form.
  const next =
    here === followed.here ? afterRead(followed, query, unsaved) : switched(followed, here, query)
  if (next !== followed) {
    setFollowed(next)
  }

  return { form: next.form, left: next.left }
}

// answeredAt is when the read last answered, with the file or a refusal.
function answeredAt(query: UseQueryResult<ConfigRead>): number {
  return Math.max(query.dataUpdatedAt, query.errorUpdatedAt)
}

// switched is where Settings stands once the server works in here: awaiting
// the read again of the configuration, unless it worked nowhere before, or
// works nowhere now.
function switched(
  followed: Followed,
  here: string | undefined,
  query: UseQueryResult<ConfigRead>,
): Followed {
  const left = followed.here
  if (left === undefined || here === undefined) {
    return { ...followed, here, awaited: null }
  }

  const awaited = { left, revision: query.data?.revision, answered: answeredAt(query) }

  return { ...followed, here, awaited }
}

// afterRead is where Settings stands once the read awaited answers: the same
// form over the same file, or a new one over another file or a refusal, saying
// so when the form held unsaved edits. Until it answers, nothing changes: the
// first answer after the switch is the read it sent, which cancels any read
// still out when it went.
function afterRead(
  followed: Followed,
  query: UseQueryResult<ConfigRead>,
  unsaved: boolean,
): Followed {
  const { awaited } = followed
  if (awaited === null || answeredAt(query) <= awaited.answered) {
    return followed
  }

  const moved = query.isError || query.data?.revision !== awaited.revision

  return {
    ...followed,
    awaited: null,
    form: moved ? followed.form + 1 : followed.form,
    left: moved && unsaved ? awaited.left : null,
  }
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
