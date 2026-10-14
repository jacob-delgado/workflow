import type { Ci, Stage } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { ActiveTask } from '@/features/tasks/ActiveTask.tsx'
import { StateMark, type MarkState } from '@/lib/StateMark.tsx'
import { cn } from '@/lib/utils.ts'
import { sectionMeta, sectionOf } from './sections.ts'

// A stage's state as the track draws its mark and says it in words.
const drawn: Record<Stage['state'], { mark: MarkState; words: string }> = {
  not_started: { mark: 'not-started', words: 'not started' },
  in_flight: { mark: 'in-flight', words: 'in flight' },
  done: { mark: 'done', words: 'done' },
  failed: { mark: 'failed', words: 'failed' },
}

// LoopTrack is the loop under the header: the checked-out branch's five
// stages, in order, each by its mark and name over a rule that fills in its
// system's hue as the stage gets done, as the terminal's spine draws them; the
// task you have started ends it. A stage done or under way wears its system's
// hue, a failed one red, and one not begun is faint. The track only shows: it
// takes no focus, and its states are said in words beside the marks.
export function LoopTrack() {
  const stages = useSnapshotStore((state) => state.snapshot?.stages)
  const ci = useSnapshotStore((state) => state.snapshot?.review.ci ?? undefined)

  if (stages === undefined) {
    return null
  }

  return (
    <div className="flex flex-wrap items-center gap-x-block gap-y-tight border-b border-border px-4 py-2">
      <ol
        aria-label="Where the checked-out branch is in the loop"
        className="flex min-w-[min(100%,32rem)] flex-[3_1_32rem] gap-item"
      >
        {stages.map((stage) => (
          <StageItem key={stage.step} stage={stage} ci={ci} />
        ))}
      </ol>
      <ActiveTask />
    </div>
  )
}

// StageItem is one stage on the track: its mark and name, its state in words
// for a screen reader, and its rule.
function StageItem({ stage, ci }: { stage: Stage; ci: Ci | undefined }) {
  const { mark, words } = drawn[stage.state]
  const checks = stage.step === 'review' && stage.state === 'in_flight' ? ci : undefined

  return (
    <li className={cn('flex min-w-0 flex-1 flex-col gap-1', hueOf(stage))}>
      <span className="flex min-w-0 items-center gap-1.5 text-xs md:text-sm">
        <StateMark state={mark} className="text-current" />
        <span className="truncate">{stage.name}</span>
        <span className="sr-only">
          : {words}
          {checks === undefined || checks.total === 0
            ? ''
            : `, ${String(checks.done)} of ${String(checks.total)} checks finished`}
        </span>
      </span>
      <span aria-hidden className="h-[3px] overflow-hidden rounded-full bg-track">
        <span
          className="block h-full origin-left bg-current motion-safe:transition-transform motion-safe:duration-500"
          style={{ transform: `scaleX(${String(filled(stage, checks))})` }}
        />
      </span>
    </li>
  )
}

// hueOf is what a stage is drawn in: its system's hue once it is under way, red
// where it failed, and the faint ink while it is not begun.
function hueOf(stage: Stage): string {
  switch (stage.state) {
    case 'not_started':
      return 'text-muted-foreground'
    case 'failed':
      return 'text-destructive'
    case 'in_flight':
    case 'done':
      return sectionMeta[sectionOf(stage.step)].hue
  }
}

// filled is how much of a stage's rule is filled, from 0 to 1: all of it once
// the stage is done, CI's share of its checks finished while a review is under
// way, and none otherwise.
function filled(stage: Stage, checks: Ci | undefined): number {
  if (stage.state === 'done') {
    return 1
  }

  return checks === undefined || checks.total === 0 ? 0 : checks.done / checks.total
}
