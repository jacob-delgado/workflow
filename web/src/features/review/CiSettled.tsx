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
// (ciFinishNotice, internal/tui/review.go) — and only when ui.notify asks to be
// told. It is a live line, mounted for the page's life whatever section is
// open, so the moment is heard where the user is: empty until there is
// something to say, and emptied when CI runs again or the branch's pull
// request changes.
export function CiSettled() {
  const settled = useSettledCi()
  const notify = useConfigRead().data?.config.ui.notify === true
  const { sigil } = useForgeWords()
  const shown = notify ? settled : null

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
// or null. It follows the snapshot as each lands, comparing one frame's CI
// with the last one's, and the same pull request's alone.
function useSettledCi(): Settled | null {
  const pull = useSnapshotStore((state) => state.snapshot?.review.pull?.number ?? null)
  const ci = useSnapshotStore((state) => state.snapshot?.review.ci?.state ?? null)
  const [seen, setSeen] = useState<Watched>({ pull, ci })
  const [settled, setSettled] = useState<Settled | null>(null)

  // Set while rendering, as React has state follow what it reads, so the frame
  // that settles CI is the one drawn saying so.
  if (seen.pull !== pull || seen.ci !== ci) {
    setSeen({ pull, ci })
    setSettled(justSettled(seen, { pull, ci }))
  }

  return settled
}

// justSettled is the pull request whose CI went from running, in the last
// frame, to passed or failed in this one, or null.
function justSettled(was: Watched, now: Watched): Settled | null {
  if (now.pull === null || was.pull !== now.pull || was.ci !== 'running') {
    return null
  }

  return now.ci === 'passed' || now.ci === 'failed' ? { pull: now.pull, ci: now.ci } : null
}
