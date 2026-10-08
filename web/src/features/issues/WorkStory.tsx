import { ChevronRight } from 'lucide-react'
import { useId, useState, type ReactNode } from 'react'
import type { CreatedWorktree, Snapshot, TaskBranch } from '@/api/generated/types.gen.ts'
import { useForgeWords, type ForgeWords } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { Button } from '@/lib/Button.tsx'
import { Meta } from '@/lib/Meta.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { capitalized, cn, plural } from '@/lib/utils.ts'
import { sectionMeta } from '@/shell/sections.ts'
import { StateMark, type MarkState } from '@/lib/StateMark.tsx'
import { useUiStore, type Section } from '@/shell/uiStore.ts'
import { checkoutBranch } from './checkoutApi.ts'
import {
  StartInWorktreeButton,
  StartRefusal,
  SwitchToWorktreeButton,
  WorktreeMadeOffer,
} from './StartInWorktree.tsx'
import { startWork } from './startWorkApi.ts'

type SnapshotStage = Snapshot['stages'][number]
type Step = SnapshotStage['step']

// How far a stage has got, drawn by its mark: not started, in flight, done or
// failed. The mark takes the hue of the system the stage belongs to, as the
// interface's spine does (internal/tui/spine.go), so a stage and the section
// it opens share a color — except a failed stage, red as the spine paints it.
type StageState = Exclude<MarkState, 'unknown'>

// A stage's state in the server's words, as the story draws it.
const drawnState: Record<SnapshotStage['state'], StageState> = {
  not_started: 'not-started',
  in_flight: 'in-flight',
  done: 'done',
  failed: 'failed',
}

// A stage's state as the story says it beside the mark.
const stateWords: Record<StageState, string> = {
  'not-started': 'not started',
  'in-flight': 'in flight',
  done: 'done',
  failed: 'failed',
}

interface Stage {
  title: string
  // detail is what the stage has come to, as facts said apart; a branch among
  // them is set in the code face (BranchName).
  detail: ReactNode[]
  section: Section
  state: StageState
}

// The issue's place in the loop: no local branch yet, a branch that is not
// checked out, or the branch on HEAD.
type Place = 'not-started' | 'elsewhere' | 'on-head'

// The loop, top to bottom, as the server sends its stages: the issue picked
// up, branched for, the work committed, the pull request opened and CI green,
// and announced. Each stage is titled and opens a section by its step; the pull
// request is named in the forge's own words — a merge request on GitLab.
function stageLook(step: Step, noun: string): Pick<Stage, 'title' | 'section'> {
  switch (step) {
    case 'issue':
      return { title: 'Issue', section: 'issues' }
    case 'branch':
      return { title: 'Branch', section: 'branch' }
    case 'commits':
      return { title: 'Changes', section: 'branch' }
    case 'review':
      return { title: capitalized(noun), section: 'review' }
    case 'announce':
      return { title: 'Announce', section: 'messaging' }
  }
}

// buildStages is the issue's story over the stages the server sends. The
// stages describe the checked-out branch, so the issue on HEAD draws each in
// the state the server read it in. An issue with no local branch has not
// started. One with a branch that is not checked out has been picked up and
// branched for, but the detail panels describe only the checked-out branch, so
// its later stages wait.
function buildStages(
  snapshot: Snapshot,
  branch: TaskBranch | undefined,
  words: ForgeWords,
): Stage[] {
  const place = placeOf(branch)
  const stateOf = stateFor(place, snapshot.stages)
  const detailOf = detailFor(place, snapshot, branch?.name ?? '', words)

  return snapshot.stages.map((stage, index) => ({
    ...stageLook(stage.step, words.noun),
    state: stateOf(stage, index),
    detail: detailOf(stage),
  }))
}

// placeOf is where the issue's work stands, by its branch.
function placeOf(branch: TaskBranch | undefined): Place {
  if (!branch) {
    return 'not-started'
  }

  return branch.current ? 'on-head' : 'elsewhere'
}

// stateFor is how far each stage has got, in the story of the issue's place:
// on HEAD, as the server read it; otherwise by where the issue stands, since
// the server's stages are the checked-out branch's — an issue with a branch
// has been picked up and branched for, the next stage is the step it is at,
// and those after it are not started.
function stateFor(
  place: Place,
  stages: SnapshotStage[],
): (stage: SnapshotStage, index: number) => StageState {
  if (place === 'on-head') {
    return (stage) => drawnState[stage.state]
  }

  const branched = (stage: SnapshotStage) =>
    place === 'elsewhere' && (stage.step === 'issue' || stage.step === 'branch')
  const at = stages.findIndex((stage) => !branched(stage))

  return (stage, index) => {
    if (branched(stage)) {
      return 'done'
    }

    return index === at ? 'in-flight' : 'not-started'
  }
}

// detailFor is what each stage has come to, in the story of the issue's place;
// branchName is the issue's branch, where it has one.
function detailFor(
  place: Place,
  snapshot: Snapshot,
  branchName: string,
  words: ForgeWords,
): (stage: SnapshotStage) => ReactNode[] {
  switch (place) {
    case 'not-started':
      return (stage) => [notStartedDetail(stage.step, words.noun, snapshot.messaging)]
    case 'elsewhere':
      return (stage) => [elsewhereDetail(stage.step, branchName, snapshot.messaging)]
    case 'on-head':
      return (stage) => onHeadDetail(stage, snapshot, words)
  }
}

// notStartedDetail is a stage's detail for an issue with no local branch yet.
function notStartedDetail(step: Step, noun: string, messaging: Snapshot['messaging']): string {
  switch (step) {
    case 'issue':
      return 'Not picked up yet'
    case 'branch':
      return 'No branch for this issue yet'
    case 'commits':
      return 'Nothing committed yet'
    case 'review':
      return `No ${noun} yet`
    case 'announce':
      return announceDetail(messaging)
  }
}

// elsewhereDetail is a stage's detail for an issue in flight on a branch that
// is not checked out: its changes and pull request are only visible from the
// checked-out branch.
function elsewhereDetail(
  step: Step,
  branchName: string,
  messaging: Snapshot['messaging'],
): ReactNode {
  switch (step) {
    case 'issue':
      return 'Picked up'
    case 'branch':
      return <BranchName key="branch" name={branchName} />
    case 'commits':
    case 'review':
      return 'Shown for the checked-out branch'
    case 'announce':
      return announceDetail(messaging)
  }
}

// onHeadDetail is a stage's detail for the issue that owns the checked-out
// branch, from what the stream says of that branch.
function onHeadDetail(stage: SnapshotStage, snapshot: Snapshot, words: ForgeWords): ReactNode[] {
  const { branch } = snapshot
  switch (stage.step) {
    case 'issue':
      return [stage.state === 'done' ? 'Picked up' : 'Not picked up yet']
    case 'branch':
      return branch.name === ''
        ? ['Not on a branch yet']
        : [<BranchName key="branch" name={branch.name} />, `${String(branch.ahead)} ahead`]
    case 'commits':
      return [changesDetail(snapshot)]
    case 'review':
      return reviewDetail(snapshot, words)
    case 'announce':
      return [stage.state === 'done' ? 'Announced' : announceDetail(snapshot.messaging)]
  }
}

// announceDetail describes the Announce stage not yet done: not announced, to
// the channel it would go to. A webhook posts where it is bound, which names no
// channel, so the stage names none.
function announceDetail(messaging: Snapshot['messaging']): string {
  if (!messaging.configured) {
    return `Not announced: ${messaging.service} is not set up`
  }
  if (messaging.channel === '') {
    return 'Not announced'
  }

  return `Not announced to ${messaging.channel}`
}

function changesDetail(snapshot: Snapshot): string {
  const { changes, branch } = snapshot
  if (changes.changes.length > 0) {
    return `${plural(changes.changes.length, 'file')} to commit`
  }
  if (branch.commits.length > 0) {
    return 'Working tree clean'
  }

  return 'Nothing committed yet'
}

function reviewDetail(snapshot: Snapshot, { noun, sigil }: ForgeWords): string[] {
  const { review } = snapshot
  if (!review.found || !review.pull) {
    return [`No ${noun} yet`]
  }

  const number = `${sigil}${String(review.pull.number)}`

  if (!review.ci) {
    return [number]
  }

  // A forge that counts no checks says so in words, as the terminal does.
  return [number, review.ci.state === 'none' ? 'No checks reported' : `CI ${review.ci.state}`]
}

// BranchName is a branch as the story names it: in the code face, as the
// Branch section's heading sets it, since it is what a reader would type.
function BranchName({ name }: { name: string }) {
  return <code>{name}</code>
}

// storyNote explains an issue that is not the checked-out one: never started, or
// in flight on a branch that is not on HEAD. The issue on HEAD needs no note —
// its stages speak for themselves.
function storyNote(branch: TaskBranch | undefined, noun: string): ReactNode {
  if (!branch) {
    return `Not in progress — its branch, changes, and ${noun} appear here once you start work on it.`
  }
  if (!branch.current) {
    return (
      <>
        In progress on <BranchName name={branch.name} /> — its changes and {noun} show when it is
        the branch you are on.
      </>
    )
  }

  return null
}

interface StoryActionProps {
  issueKey: string
  branch: TaskBranch | undefined
  outcome: Teller
}

// StoryAction is what moves an issue along from here: starting work on it, in
// place or in a new worktree; checking its branch out; or switching to the
// worktree that has it checked out, since git will not check it out twice,
// or saying how to free it when that worktree is gone. A worktree just made is
// offered to switch to, and stays offered when the snapshot then shows its
// branch, until a switch makes that branch the one checked out.
function StoryAction({ issueKey, branch, outcome }: StoryActionProps) {
  const [made, setMade] = useState<CreatedWorktree | null>(null)

  // Once the switch has made the branch the one checked out, there is nothing
  // left to offer.
  if (made !== null && branch?.current !== true) {
    return <WorktreeMadeOffer issueKey={issueKey} worktree={made} outcome={outcome} />
  }

  if (branch === undefined) {
    return (
      <div className="flex flex-wrap items-start gap-item">
        <StartWorkButton issueKey={issueKey} outcome={outcome} />
        <StartInWorktreeButton issueKey={issueKey} outcome={outcome} onMade={setMade} />
      </div>
    )
  }

  if (branch.current) {
    return null
  }

  if (branch.worktree && branch.worktree_missing) {
    return (
      <p className="text-sm text-foreground">
        Its worktree at <span className="font-mono">{branch.worktree_shown}</span> is gone, and git
        holds the branch for it until <span className="font-mono">git worktree prune</span> frees
        it.
      </p>
    )
  }

  return branch.worktree ? (
    <SwitchToWorktreeButton
      dir={branch.worktree}
      shown={branch.worktree_shown ?? branch.worktree}
      outcome={outcome}
    />
  ) : (
    <CheckoutButton branch={branch.name} outcome={outcome} />
  )
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
  const at = stages.findIndex((stage) => stage.state !== 'done')
  const note = storyNote(branch, words.noun)

  return (
    <div className="flex flex-col gap-group">
      {note === null ? null : <p className="text-sm text-muted-foreground">{note}</p>}
      <StoryAction issueKey={issueKey} branch={branch} outcome={outcome} />
      <OutcomeLine said={outcome.said} />
      <ol className="flex flex-col">
        {stages.map((stage, index) => (
          <StoryStage
            key={stage.title}
            stage={stage}
            current={index === at}
            last={index === stages.length - 1}
            onOpen={setSection}
          />
        ))}
      </ol>
    </div>
  )
}

interface StoryStageProps {
  stage: Stage
  // current is whether the stage is the step the work is at: the first not
  // done.
  current: boolean
  last: boolean
  onOpen: (section: Section) => void
}

// StoryStage is one stage of the story: the mark of how far it has come, in
// its system's hue, on the line down to the next, and the stage itself as a
// control that opens its section — a chevron after its title says so at rest, and
// its description names the section for a screen reader. The first stage not
// done is the current step: where the work is at.
function StoryStage({ stage, current, last, onOpen }: StoryStageProps) {
  const opens = useId()

  return (
    <li className="flex gap-item">
      <div className="flex flex-col items-center gap-tight pt-1.5">
        <StateMark
          state={stage.state}
          className={cn('size-4', stage.state !== 'failed' && sectionMeta[stage.section].hue)}
        />
        {last ? null : <span className="w-px flex-1 bg-border" />}
      </div>
      {/* Not a Button: a stage of the story that opens its section, drawn as the stage. */}
      <button
        type="button"
        aria-describedby={opens}
        aria-current={current ? 'step' : undefined}
        onClick={() => {
          onOpen(stage.section)
        }}
        className="flex flex-1 flex-col gap-tight rounded-md px-2 pt-0.5 pb-block text-left hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        <span className="flex items-center gap-1 text-sm font-medium">
          {stage.title}
          <ChevronRight aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        </span>
        <span className="sr-only">{stateWords[stage.state]}</span>
        <Meta className="text-sm text-muted-foreground">{stage.detail}</Meta>
      </button>
      <span id={opens} hidden>
        Opens {sectionMeta[stage.section].label}
      </span>
    </li>
  )
}

// CheckoutButton switches the working tree to a branch that is not on HEAD.
// Where it went is said in the story's outcome; a refusal — a dirty tree — is
// shown inline for the user to act on.
function CheckoutButton({ branch, outcome }: { branch: string; outcome: Teller }) {
  const { state, error, run } = useAsyncAction(() => checkoutBranch(branch), {
    fallback:
      'The branch was not switched to. Try again, or switch to it from a terminal to see why.',
    done: (switched) => `Switched to ${switched.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="secondary"
        held={state === 'running'}
        onClick={() => {
          void run()
        }}
        className="self-start"
      >
        {state === 'running' ? 'Switching…' : 'Switch branch'}
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
  const start = useAsyncAction((fetch: boolean) => startWork(issueKey, fetch), {
    fallback: `No branch was made for ${issueKey}. Try again, or run workflow branch ${issueKey} from a terminal.`,
    done: (started) => `Started work on ${started.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="primary"
        held={start.state === 'running'}
        onClick={() => {
          void start.run(true)
        }}
        className="self-start"
      >
        {start.state === 'running' ? 'Starting…' : 'Start work'}
      </Button>
      <StartRefusal start={start} />
    </div>
  )
}
