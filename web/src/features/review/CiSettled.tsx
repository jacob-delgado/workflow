import { useState } from 'react'
import type { CiState } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useConfigRead } from '@/features/settings/configApi.ts'
import { ciMark, StateMark } from '@/lib/StateMark.tsx'

// Watched is the branch's pull request, by number, and how its CI stood when
// the stream last said.
interface Watched {
  pull: number | null
  ci: CiState | null
}

// Settled is a pull request whose CI has just finished, and how.
interface Settled {
  pull: number
  ci: 'passed' | 'failed'
}

// CiSettled says, once, that CI on the branch's pull request has just gone from
// running to passed or failed — the moment the terminal rings for
// (ciFinishNotice, internal/tui/ci.go) — and only when ui.notify asks to be
// told. It is a live line, mounted for the page's life whatever section is
// open, so the moment is heard where the user is: empty until there is
// something to say, and emptied when CI runs again or the branch's pull
// request changes.
export function CiSettled() {
  const shown = useSettledCi(useConfigRead().data?.config.ui.notify)
  const { sigil } = useForgeWords()

  return (
    <p role="status" className="flex items-center gap-1.5 text-xs empty:sr-only">
      {shown === null ? null : (
        <>
          <StateMark state={ciMark[shown.ci]} className="size-3" />
          {`CI ${shown.ci} on ${sigil}${String(shown.pull)}.`}
        </>
      )}
    </p>
  )
}

// useSettledCi is the pull request whose CI the stream has just seen settle,
// or null, while notify asks to be told. It follows the snapshot as each
// lands, comparing one frame's CI with the last one's read, and the same pull
// request's alone. CI that settles while notify is off is never told, so
// turning it on later does not tell an old settle as news; one that settles
// before the configuration is read is told once it is, if it asks.
function useSettledCi(notify: boolean | undefined): Settled | null {
  const pull = useSnapshotStore((state) => state.snapshot?.review.pull?.number ?? null)
  const ci = useSnapshotStore((state) => state.snapshot?.review.ci?.state ?? null)
  const unread = useSnapshotStore((state) => (state.snapshot?.review.ci_error ?? null) !== null)
  const [seen, setSeen] = useState<Watched>({ pull, ci })
  const [settled, setSettled] = useState<Settled | null>(null)

  // Set while rendering, as React has state follow what it reads, so the frame
  // that settles CI is the one drawn saying so. A frame whose CI could not be
  // read says nothing of how it stands, so it is not taken for news, and the
  // frame after compares with how CI stood before it.
  const unreadOfSeen = unread && ci === null && pull === seen.pull
  if (!unreadOfSeen && (seen.pull !== pull || seen.ci !== ci)) {
    setSeen({ pull, ci })
    setSettled(justSettled(seen, { pull, ci }))
  }
  if (notify === false && settled !== null) {
    setSettled(null)
  }

  return notify === true ? settled : null
}

// justSettled is the pull request whose CI went from running, in the last
// frame, to passed or failed in this one, or null.
function justSettled(was: Watched, now: Watched): Settled | null {
  if (now.pull === null || was.pull !== now.pull || was.ci !== 'running') {
    return null
  }

  return now.ci === 'passed' || now.ci === 'failed' ? { pull: now.pull, ci: now.ci } : null
}
