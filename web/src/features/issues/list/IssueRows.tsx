import { useEffect, useRef, type RefObject } from 'react'
import type { Issue, TaskBranch, TasksSummary } from '@/api/generated/types.gen.ts'
import { checkoutBranch } from '@/features/issues/checkoutApi.ts'
import { IssueStatus } from '@/features/issues/IssueStatus.tsx'
import { shownKey } from '@/features/issues/issuePlaces.ts'
import { issueTaskMark, linkedTo } from '@/features/tasks/taskWords.ts'
import { Button } from '@/lib/Button.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { StateMark } from '@/lib/StateMark.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn } from '@/lib/utils.ts'
import { useUiStore } from '@/shell/uiStore.ts'

// RowFacts are what the rows mark beside each issue: its local branches, and
// how its tasks stand.
export interface RowFacts {
  // The local branches by the issue they name, most-recently-committed first.
  branchesByKey: Map<string, TaskBranch[]>
  // What the stream carries of your tasks, whose marks the rows draw once it
  // says Taskwarrior can be asked.
  tasks: TasksSummary
}

// groupBranchesByKey groups the local branches by the issue they name, keeping
// their order (most-recently-committed first) so the first is the most recent.
export function groupBranchesByKey(branches: TaskBranch[]): Map<string, TaskBranch[]> {
  const byKey = new Map<string, TaskBranch[]>()
  for (const branch of branches) {
    const existing = byKey.get(branch.issue_key)
    if (existing) {
      existing.push(branch)
    } else {
      byKey.set(branch.issue_key, [branch])
    }
  }

  return byKey
}

interface IssueRowsProps {
  issues: Issue[]
  facts: RowFacts
  // Each listed row's button by issue key, so focus can be handed to a row.
  rowRefs: RefObject<Map<string, HTMLButtonElement>>
  outcome: Teller
}

// IssueRows is the list of issues, each row selecting its issue, marking it in
// flight and how its tasks stand, and offering its branch to switch to.
export function IssueRows({ issues, facts, rowRefs, outcome }: IssueRowsProps) {
  const selected = useUiStore((state) => state.selectedIssue)
  const pane = useSelectedRowInView(rowRefs, selected)

  return (
    <ul
      ref={pane}
      aria-label="Issues"
      className="flex max-h-80 min-h-0 flex-col gap-tight overflow-y-auto rounded-lg border border-border p-1 lg:max-h-none lg:rounded-none lg:border-0"
    >
      {issues.map((issue) => (
        <IssueRow
          key={issue.key}
          issue={issue}
          facts={facts}
          rowRefs={rowRefs}
          selected={issue.key === selected}
          outcome={outcome}
        />
      ))}
    </ul>
  )
}

interface IssueRowProps extends Omit<IssueRowsProps, 'issues'> {
  issue: Issue
  selected: boolean
}

// IssueRow is one issue of the list. An issue can have more than one local
// branch; it is on HEAD when any of them is, and check-out targets the most
// recent one.
function IssueRow({ issue, facts, rowRefs, selected, outcome }: IssueRowProps) {
  const selectIssue = useUiStore((state) => state.selectIssue)
  const issueBranches = facts.branchesByKey.get(issue.key) ?? []
  const newest = issueBranches[0]
  const onHead = issueBranches.some((branch) => branch.current)
  const taskMark = facts.tasks.available
    ? issueTaskMark(linkedTo(facts.tasks.linked, issue.key))
    : undefined

  return (
    <li
      className={cn(
        'flex flex-col rounded-md border border-transparent hover:bg-accent motion-safe:transition-colors',
        selected && 'border-border bg-accent',
      )}
    >
      {/* Not a Button: a row of the list that selects its issue, drawn as the row. */}
      <button
        ref={(element) => {
          if (element !== null) {
            rowRefs.current.set(issue.key, element)
          }

          return () => {
            rowRefs.current.delete(issue.key)
          }
        }}
        type="button"
        aria-current={selected ? true : undefined}
        onClick={() => {
          selectIssue(issue.key)
        }}
        className="flex flex-col gap-tight rounded-md px-3 py-2 text-left focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        <span className="flex flex-wrap items-center gap-item">
          <span className="text-xs text-muted-foreground tabular-nums">{shownKey(issue)}</span>
          <IssueStatus category={issue.status_category} label={issue.status} />
          <RowMarks inFlight={newest !== undefined} taskMark={taskMark} />
        </span>
        <span className="text-sm">{issue.summary}</span>
      </button>
      {newest && !onHead ? (
        <RowCheckout branch={newest.name} issueKey={issue.key} outcome={outcome} />
      ) : null}
    </li>
  )
}

// RowMarks are what a row marks at its end: the issue in flight, when a branch
// names it, and how its tasks stand, when Taskwarrior can be asked and a task
// is linked to it — each by its mark, beside the words for it. They sit
// together, so where the row is too narrow for them they move to a line of
// their own as one.
function RowMarks({
  inFlight,
  taskMark,
}: {
  inFlight: boolean
  taskMark: ReturnType<typeof issueTaskMark>
}) {
  if (!inFlight && taskMark === undefined) {
    return null
  }

  return (
    <span className="ml-auto flex items-center gap-item text-xs whitespace-nowrap text-muted-foreground">
      {inFlight ? (
        <span className="flex items-center gap-1">
          <StateMark state="in-flight" className="size-3 text-git" />
          <span>in flight</span>
        </span>
      ) : null}
      {taskMark === undefined ? null : (
        <span className="flex items-center gap-1">
          <StateMark state={taskMark.state} className="size-3 text-taskwarrior" />
          <span>{taskMark.words}</span>
        </span>
      )}
    </span>
  )
}

// useSelectedRowInView brings the selected issue's row into view in the list's
// pane, scrolling the pane alone and only as far as it must. The selection
// outlives a change of section and the pane's scroll does not, so the list
// would reopen at its top with the issue its detail shows out of sight.
function useSelectedRowInView(
  rowRefs: RefObject<Map<string, HTMLButtonElement>>,
  selected: string | null,
) {
  const pane = useRef<HTMLUListElement>(null)

  useEffect(() => {
    const list = pane.current
    const row = selected === null ? undefined : rowRefs.current.get(selected)
    if (list === null || row === undefined) {
      return
    }

    const top = row.getBoundingClientRect().top - list.getBoundingClientRect().top - list.clientTop
    const bottom = top + row.offsetHeight
    const inset = parseFloat(getComputedStyle(list).paddingTop)
    if (top < 0) {
      list.scrollTop += top - inset
    } else if (bottom > list.clientHeight) {
      list.scrollTop += bottom - list.clientHeight + inset
    }
  }, [rowRefs, selected])

  return pane
}

interface RowCheckoutProps {
  branch: string
  issueKey: string
  outcome: Teller
}

// RowCheckout switches to an in-flight issue's branch from the list, so moving
// between tasks does not need the detail panel first. Where it went is said in
// the panel's outcome; a refusal — a dirty tree — is shown inline. It is the
// row's bottom line, inside the row's box and under its summary, so a row that
// offers it is as wide as one that does not and the list keeps one right edge.
function RowCheckout({ branch, issueKey, outcome }: RowCheckoutProps) {
  const { state, error, run } = useAsyncAction(() => checkoutBranch(branch), {
    fallback:
      'The branch was not switched to. Try again, or switch to it from a terminal to see why.',
    done: (switched) => `Switched to ${switched.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col items-start gap-tight px-3 pb-2">
      <Button
        variant="secondary"
        size="sm"
        aria-label={
          state === 'running'
            ? `Switching branch for ${issueKey}…`
            : `Switch branch for ${issueKey}`
        }
        held={state === 'running'}
        onClick={() => {
          void run()
        }}
      >
        {state === 'running' ? 'Switching…' : 'Switch branch'}
      </Button>
      {state === 'error' ? (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}
