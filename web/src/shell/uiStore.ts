import { create } from 'zustand'
import type { Issue } from '@/api/generated/types.gen.ts'
import type { Facet } from '@/features/reviewqueue/reviewFacets.ts'
import type { ReviewOrder } from '@/features/reviewqueue/reviewOrder.ts'
import type { TaskFacet } from '@/features/tasks/taskFacets.ts'
import type { Period } from '@/features/summary/civilDate.ts'
import type { TaskOrder } from '@/features/tasks/taskOrder.ts'

// The cockpit's sections, in nav order: the interface's panes, in its order —
// its Commits pane is part of Branch here — then Settings. The work story
// (Issue → Branch → Commits → Review → the messaging service) is reachable
// from an issue in the Issues section; the rest are direct views.
export const sections = [
  'issues',
  'branch',
  'review',
  'messaging',
  'reviews',
  'tasks',
  'summary',
  'repositories',
  'settings',
] as const

export type Section = (typeof sections)[number]

// IssueRef is an issue by its key and the tracker it lives in, as the server
// names it: a key alone says nothing the page should read its tracker from.
export type IssueRef = Pick<Issue, 'key' | 'tracker'>

interface UiState {
  section: Section
  setSection: (section: Section) => void
  // The issue whose detail the Issues section shows, or null for none.
  selectedIssue: IssueRef | null
  selectIssue: (issue: IssueRef | null) => void
  // The issue view the stream carries, by name, or null for the server's default.
  view: string | null
  setView: (view: string | null) => void
  // How the review queue is listed and narrowed, kept while you visit other
  // sections, as the terminal keeps them across its panes.
  reviewOrder: ReviewOrder
  setReviewOrder: (order: ReviewOrder) => void
  reviewFilter: Facet[]
  pickReviewFilter: (pick: (picked: Facet[]) => Facet[]) => void
  // How the Tasks list is sorted and narrowed, kept the same way.
  taskOrder: TaskOrder
  setTaskOrder: (order: TaskOrder) => void
  taskFilter: TaskFacet[]
  pickTaskFilter: (pick: (picked: TaskFacet[]) => TaskFacet[]) => void
  // The period the Summary shows, or null for the previous working day, which
  // the server picks: kept while you visit other sections, as the terminal's
  // pane keeps its period.
  summaryPeriod: Period | null
  setSummaryPeriod: (period: Period | null) => void
}

// Client UI state — which section is showing, which issue is selected, which
// view the list is of, how the review queue and the Tasks list are sorted and
// narrowed, and the period the Summary shows — shared by the nav rail and the
// content panes without threading props between them.
export const useUiStore = create<UiState>((set) => ({
  section: 'issues',
  setSection: (section) => {
    set({ section })
  },
  selectedIssue: null,
  selectIssue: (issue) => {
    set({ selectedIssue: issue === null ? null : { key: issue.key, tracker: issue.tracker } })
  },
  view: null,
  setView: (view) => {
    set({ view })
  },
  reviewOrder: 'oldest',
  setReviewOrder: (reviewOrder) => {
    set({ reviewOrder })
  },
  reviewFilter: [],
  pickReviewFilter: (pick) => {
    set((state) => ({ reviewFilter: pick(state.reviewFilter) }))
  },
  taskOrder: 'urgency',
  setTaskOrder: (taskOrder) => {
    set({ taskOrder })
  },
  taskFilter: [],
  pickTaskFilter: (pick) => {
    set((state) => ({ taskFilter: pick(state.taskFilter) }))
  },
  summaryPeriod: null,
  setSummaryPeriod: (summaryPeriod) => {
    set({ summaryPeriod })
  },
}))
