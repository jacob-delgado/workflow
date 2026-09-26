import type { Snapshot, TaskBranch } from '@/api/generated/types.gen.ts'
import { useForgeWords, type ForgeWords } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { capitalized, cn } from '@/lib/utils.ts'
import { sectionMeta } from '@/shell/sections.ts'
import { StateMark, type MarkState } from '@/shell/StateMark.tsx'
import { useUiStore, type Section } from '@/shell/uiStore.ts'
import { checkoutBranch } from './checkoutApi.ts'
import { startWork } from './startWorkApi.ts'

type StageState = 'done' | 'failed' | 'active' | 'upcoming'

// A stage's mark: done, failed, the stage the work is at, and the stages still
// to come. It takes the hue of the system the stage belongs to, as the
// interface's spine does (internal/tui/spine.go), so a stage and the section
// it opens share a color — except a failed stage, red as the spine paints it.
const stageMark: Record<StageState, MarkState> = {
  done: 'done',
  failed: 'failed',
  active: 'in-flight',
  upcoming: 'not-started',
}

// How far a stage has got, read from the stream by the rules internal/progress
// writes for the spine and `workflow status`; WorkStory.stages.test.tsx holds
// the two equal. Which pending stage the work is at is the story's own reading
// (stageState).
type Reached = 'done' | 'failed' | 'pending'

interface Stage {
  title: string
  detail: string
  section: Section
  reached: Reached
}

// The loop, top to bottom: branch for the issue, commit the work, open the pull
// request and get CI green, announce it. An issue with no local branch has not
// started. One with a branch that is not checked out has begun, but the detail
// panels describe only the checked-out branch, so its later stages wait. The
// issue on HEAD reads every stage's state from the stream. The pull request is
// named in the forge's own words — a merge request on GitLab.
function buildStages(
  snapshot: Snapshot,
  branch: TaskBranch | undefined,
  words: ForgeWords,
): Stage[] {
  if (!branch) {
    return notStartedStages(words.noun)
  }
  if (!branch.current) {
    return offHeadStages(branch.name, words.noun)
  }

  return onHeadStages(snapshot, words)
}

// notStartedStages is the story for an issue with no local branch yet.
function notStartedStages(noun: string): Stage[] {
  return [
    {
      title: 'Branch',
      section: 'branch',
      reached: 'pending',
      detail: 'No branch for this issue yet',
    },
    { title: 'Changes', section: 'branch', reached: 'pending', detail: 'Nothing committed yet' },
    { title: capitalized(noun), section: 'review', reached: 'pending', detail: `No ${noun} yet` },
    { title: 'Announce', section: 'messaging', reached: 'pending', detail: 'Not announced' },
  ]
}

// offHeadStages is the story for an in-flight issue whose branch is not checked
// out: the branch exists, but its changes and pull request are only visible from
// the checked-out branch, so those stages stay pending here.
function offHeadStages(branchName: string, noun: string): Stage[] {
  const elsewhere = 'Shown for the checked-out branch'

  return [
    { title: 'Branch', section: 'branch', reached: 'done', detail: branchName },
    { title: 'Changes', section: 'branch', reached: 'pending', detail: elsewhere },
    { title: capitalized(noun), section: 'review', reached: 'pending', detail: elsewhere },
    { title: 'Announce', section: 'messaging', reached: 'pending', detail: 'Not announced' },
  ]
}

// onHeadStages is the full story for the issue that owns the checked-out branch,
// with each stage's state read from the stream.
function onHeadStages(snapshot: Snapshot, words: ForgeWords): Stage[] {
  const { branch, messaging } = snapshot

  return [
    {
      title: 'Branch',
      section: 'branch',
      reached: branch.name === '' ? 'pending' : 'done',
      detail:
        branch.name === ''
          ? 'Not on a branch yet'
          : `${branch.name} · ${String(branch.ahead)} ahead`,
    },
    {
      title: 'Changes',
      section: 'branch',
      // Done on a commit, as progress.commitState reads it: files still to
      // commit do not undo one already made, and a fresh branch with nothing
      // committed is still at this stage however clean its tree.
      reached: branch.commits.length > 0 ? 'done' : 'pending',
      detail: changesDetail(snapshot),
    },
    {
      title: capitalized(words.noun),
      section: 'review',
      reached: reviewReached(snapshot),
      detail: reviewDetail(snapshot, words),
    },
    {
      title: 'Announce',
      section: 'messaging',
      reached: 'pending',
      detail: announceDetail(messaging),
    },
  ]
}

// reviewReached follows the pull request as progress.reviewState does. A merged
// one's review is over, and one closed without merging counts as none. Changes
// asked for fail the stage as a CI failure does, both something to go back to;
// only a passed CI reads done, so a forge with no CI leaves it in flight. A
// draft reads as any pull request.
function reviewReached({ review }: Snapshot): Reached {
  const { pull, ci } = review
  if (!review.found || !pull || pull.state === 'closed') {
    return 'pending'
  }
  if (pull.state === 'merged') {
    return 'done'
  }
  if (ci?.state === 'failed' || pull.changes_requested) {
    return 'failed'
  }

  return ci?.state === 'passed' ? 'done' : 'pending'
}

// announceDetail describes the Announce stage. A webhook service has no channel
// of its own, so a configured one falls through to naming the service rather
// than a channel.
function announceDetail(messaging: Snapshot['messaging']): string {
  if (!messaging.configured) {
    return `${messaging.service} not configured`
  }
  if (messaging.channel === '') {
    return `Announce to ${messaging.service}`
  }

  return `Announce to ${messaging.channel}`
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

function reviewDetail(snapshot: Snapshot, { noun, sigil }: ForgeWords): string {
  const { review } = snapshot
  if (!review.found || !review.pull) {
    return `No ${noun} yet`
  }

  const number = `${sigil}${String(review.pull.number)}`

  return review.ci ? `${number} · CI ${review.ci.state}` : number
}

function stageState(stage: Stage, index: number, activeIndex: number): StageState {
  if (stage.reached !== 'pending') {
    return stage.reached
  }
  if (index === activeIndex) {
    return 'active'
  }

  return 'upcoming'
}

// storyNote explains an issue that is not the checked-out one: never started, or
// in flight on a branch that is not on HEAD. The issue on HEAD needs no note —
// its stages speak for themselves.
function storyNote(branch: TaskBranch | undefined, noun: string): string | null {
  if (!branch) {
    return `Not in progress — its branch, changes, and ${noun} appear here once you start work on it.`
  }
  if (!branch.current) {
    return `In progress on ${branch.name} — its changes and ${noun} show when it is the branch you are on.`
  }

  return null
}

// WorkStory is an issue's stages, with the start or the check-out that moves it
// along and the line that says what that did — which stays when the snapshot
// showing the new branch takes the button away.
export function WorkStory({ issueKey }: { issueKey: string }) {
  const snapshot = useSnapshotStore((state) => state.snapshot)
  const setSection = useUiStore((state) => state.setSection)
  const words = useForgeWords()
  const outcome = useOutcome()

  if (!snapshot) {
    return null
  }

  const branch = snapshot.branches.find((entry) => entry.issue_key === issueKey)
  const stages = buildStages(snapshot, branch, words)
  const activeIndex = stages.findIndex((stage) => stage.reached !== 'done')
  const note = storyNote(branch, words.noun)

  return (
    <div className="flex flex-col gap-group">
      {note === null ? null : <p className="text-sm text-muted-foreground">{note}</p>}
      {branch === undefined ? <StartWorkButton issueKey={issueKey} outcome={outcome} /> : null}
      {branch && !branch.current ? <CheckoutButton branch={branch.name} outcome={outcome} /> : null}
      <OutcomeLine said={outcome.said} />
      <ol className="flex flex-col">
        {stages.map((stage, index) => {
          const state = stageState(stage, index, activeIndex)
          const last = index === stages.length - 1

          return (
            <li key={stage.title} className="flex gap-item">
              <div className="flex flex-col items-center gap-tight pt-1.5">
                <StateMark
                  state={stageMark[state]}
                  className={cn('size-4', state !== 'failed' && sectionMeta[stage.section].hue)}
                />
                {last ? null : <span className="w-px flex-1 bg-border" />}
              </div>
              <button
                type="button"
                onClick={() => {
                  setSection(stage.section)
                }}
                className="flex flex-1 flex-col gap-tight rounded-md px-2 pt-0.5 pb-block text-left hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
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

// CheckoutButton switches the working tree to a branch that is not on HEAD.
// Where it went is said in the story's outcome; a refusal — a dirty tree — is
// shown inline for the user to act on.
function CheckoutButton({ branch, outcome }: { branch: string; outcome: Teller }) {
  const { state, error, run } = useAsyncAction(() => checkoutBranch(branch), {
    fallback:
      'The branch was not checked out. Try again, or switch to it from a terminal to see why.',
    done: (checkedOut) => `Checked out ${checkedOut.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="secondary"
        disabled={state === 'running'}
        onClick={() => {
          void run()
        }}
        className="self-start"
      >
        {state === 'running' ? 'Checking out…' : 'Check out this branch'}
      </Button>
      {state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}

// StartWorkButton creates and switches to a branch for a not-started issue. The
// branch it made is said in the story's outcome; a refusal (a branch already
// exists) is shown inline.
function StartWorkButton({ issueKey, outcome }: { issueKey: string; outcome: Teller }) {
  const { state, error, run } = useAsyncAction(() => startWork(issueKey), {
    fallback: `No branch was made for ${issueKey}. Try again, or run workflow branch ${issueKey} from a terminal.`,
    done: (started) => `Started work on ${started.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="primary"
        disabled={state === 'running'}
        onClick={() => {
          void run()
        }}
        className="self-start"
      >
        {state === 'running' ? 'Starting…' : 'Start work'}
      </Button>
      {state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}
