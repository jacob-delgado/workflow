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

// The current branch belongs to one issue — its key is in the branch name, by
// the tool's own convention (fix/PROJ-412-slug). Only that issue has work in
// flight; every other issue's story is still to begin.
function branchIsFor(branchName: string, issueKey: string): boolean {
  return new RegExp(`(^|[^A-Za-z0-9])${issueKey}([^A-Za-z0-9]|$)`).test(branchName)
}

// The loop, top to bottom: branch for the issue, commit the work, open the pull
// request and get CI green, announce it. For the issue that owns the current
// branch each stage's "done" is read from the streamed state; for any other
// issue nothing has started yet.
function buildStages(snapshot: Snapshot, started: boolean): Stage[] {
  if (!started) {
    return [
      { title: 'Branch', section: 'branch', done: false, detail: 'No branch for this issue yet' },
      { title: 'Changes', section: 'branch', done: false, detail: 'Nothing committed yet' },
      { title: 'Pull request', section: 'review', done: false, detail: 'No pull request yet' },
      { title: 'Announce', section: 'slack', done: false, detail: 'Not announced' },
    ]
  }

  const { branch, changes, slack } = snapshot

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
      // Committed, not merely clean: a fresh branch with nothing committed is
      // still at this stage, so it needs a clean tree AND at least one commit.
      done: changes.changes.length === 0 && branch.commits.length > 0,
      detail: changesDetail(snapshot),
    },
    {
      title: 'Pull request',
      section: 'review',
      done: pullRequestDone(snapshot),
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

// Done once the pull request is open, ready, and not held up by CI. A forge
// with no CI (state none, or absent) does not keep it from done — only a running
// or failed check does.
function pullRequestDone({ review }: Snapshot): boolean {
  return (
    review.found &&
    review.pull != null &&
    !review.pull.draft &&
    review.ci?.state !== 'running' &&
    review.ci?.state !== 'failed'
  )
}

function changesDetail(snapshot: Snapshot): string {
  const { changes, branch } = snapshot
  if (changes.changes.length > 0) {
    return `${String(changes.changes.length)} file(s) to commit`
  }
  if (branch.commits.length > 0) {
    return 'Working tree clean'
  }

  return 'Nothing committed yet'
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

export function WorkStory({ issueKey }: { issueKey: string }) {
  const snapshot = useSnapshotStore((state) => state.snapshot)
  const setSection = useUiStore((state) => state.setSection)

  if (!snapshot) {
    return null
  }

  const started = branchIsFor(snapshot.branch.name, issueKey)
  const stages = buildStages(snapshot, started)
  const activeIndex = stages.findIndex((stage) => !stage.done)

  return (
    <div className="flex flex-col gap-3">
      {started ? null : (
        <p className="text-sm text-muted-foreground">
          Not in progress — its branch, changes, and pull request appear here once you pick it up.
        </p>
      )}
      <ol className="flex flex-col">
        {stages.map((stage, index) => {
          const state = stageState(stage, index, activeIndex)
          const last = index === stages.length - 1

          return (
            <li key={stage.title} className="flex gap-3">
              <div className="flex flex-col items-center">
                <StageMarker state={state} />
                {last ? null : (
                  <span
                    className={cn('w-0.5 flex-1', stage.done ? 'bg-success/40' : 'bg-border')}
                  />
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
    </div>
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
