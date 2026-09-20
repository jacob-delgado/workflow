import { Check } from 'lucide-react'
import type { Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'
import { useUiStore, type Section } from '@/shell/uiStore.ts'

type StageState = 'done' | 'active' | 'upcoming'

interface Stage {
  title: string
  detail: string
  section: Section
  done: boolean
}

// The loop, top to bottom: branch for the issue, commit the work, open the pull
// request and get CI green, announce it. Each stage's "done" is read from the
// streamed state, and the first stage that is not done is the one in progress.
function buildStages(snapshot: Snapshot): Stage[] {
  const { branch, changes, review, slack } = snapshot

  return [
    {
      title: 'Branch',
      section: 'branch',
      done: branch.name !== '',
      detail:
        branch.name === ''
          ? 'Not on a branch yet'
          : `${branch.name} · ${String(branch.ahead)} ahead`,
    },
    {
      title: 'Changes',
      section: 'branch',
      done: changes.changes.length === 0,
      detail:
        changes.changes.length === 0
          ? 'Working tree clean'
          : `${String(changes.changes.length)} file(s) to commit`,
    },
    {
      title: 'Pull request',
      section: 'review',
      done: review.found && review.ci?.state === 'passed',
      detail: reviewDetail(snapshot),
    },
    {
      title: 'Announce',
      section: 'slack',
      done: false,
      detail: slack.channel === '' ? 'Slack not configured' : `Post to ${slack.channel}`,
    },
  ]
}

function reviewDetail(snapshot: Snapshot): string {
  const { review } = snapshot
  if (!review.found || !review.pull) {
    return 'No pull request yet'
  }

  const number = String(review.pull.number)

  return review.ci ? `#${number} · CI ${review.ci.state}` : `#${number}`
}

function stageState(stage: Stage, index: number, activeIndex: number): StageState {
  if (stage.done) {
    return 'done'
  }
  if (index === activeIndex) {
    return 'active'
  }

  return 'upcoming'
}

export function WorkStory() {
  const snapshot = useSnapshotStore((state) => state.snapshot)
  const setSection = useUiStore((state) => state.setSection)

  if (!snapshot) {
    return null
  }

  const stages = buildStages(snapshot)
  const activeIndex = stages.findIndex((stage) => !stage.done)

  return (
    <ol className="flex flex-col">
      {stages.map((stage, index) => {
        const state = stageState(stage, index, activeIndex)
        const last = index === stages.length - 1

        return (
          <li key={stage.title} className="flex gap-3">
            <div className="flex flex-col items-center">
              <StageMarker state={state} />
              {last ? null : (
                <span className={cn('w-0.5 flex-1', stage.done ? 'bg-success/40' : 'bg-border')} />
              )}
            </div>
            <button
              type="button"
              onClick={() => {
                setSection(stage.section)
              }}
              className="flex flex-1 flex-col gap-0.5 rounded-md px-2 pt-0.5 pb-6 text-left hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
            >
              <span className="font-medium">{stage.title}</span>
              <span className="sr-only">{state}</span>
              <span className="text-sm text-muted-foreground">{stage.detail}</span>
            </button>
          </li>
        )
      })}
    </ol>
  )
}

function StageMarker({ state }: { state: StageState }) {
  if (state === 'done') {
    return (
      <span className="flex size-6 items-center justify-center rounded-full bg-success text-success-foreground">
        <Check aria-hidden className="size-3.5" />
      </span>
    )
  }
  if (state === 'active') {
    return (
      <span className="flex size-6 items-center justify-center rounded-full border-2 border-primary">
        <span className="size-2 rounded-full bg-primary" />
      </span>
    )
  }

  return <span className="size-6 rounded-full border-2 border-border" />
}
