import type { IssuesPage, Problem, TaskBranch, TasksSummary } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'

// ListedIssues is the page of issues the list draws for a view, and where it
// came from: the stream's snapshot for the view; the issues the store kept for
// it last time, sent ahead of the first snapshot; or neither yet, while a
// switch to the view is under way. With it come why the page could not be
// read, and what the rows' marks are read from.
export interface ListedIssues {
  from: 'stream' | 'seed' | 'switching'
  page: IssuesPage
  unread: Problem | null
  branches: TaskBranch[]
  tasks: TasksSummary
}

// emptyPage is the page listed before any is known.
const emptyPage: IssuesPage = { issues: [], total: 0, start_at: 0, unavailable: [] }

// unmarked is what the marks read before the first snapshot: no branch in
// flight, and no task asked about, so no row is marked.
const unmarked: Pick<ListedIssues, 'branches' | 'tasks'> = {
  branches: [],
  tasks: { available: false, reason: '', linked: [] },
}

// useListedIssues is the page the Issues list draws for view. The marks come
// from the last snapshot, whichever view it read, since branches and tasks
// belong to none.
export function useListedIssues(view: string | null): ListedIssues {
  const snapshot = useSnapshotStore((state) => state.snapshot)
  const streamedView = useSnapshotStore((state) => state.view)
  const seed = useSnapshotStore((state) => state.seed)
  const marks =
    snapshot === null ? unmarked : { branches: snapshot.branches, tasks: snapshot.tasks }

  if (snapshot !== null && streamedView === view) {
    return {
      from: 'stream',
      page: snapshot.issues,
      unread: snapshot.problems?.issues ?? null,
      ...marks,
    }
  }

  if (seed?.view === view) {
    return { from: 'seed', page: seed.page, unread: null, ...marks }
  }

  return { from: 'switching', page: snapshot?.issues ?? emptyPage, unread: null, ...marks }
}
