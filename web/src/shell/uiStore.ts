import { create } from 'zustand'

// The cockpit's sections, in nav order. The work story (Branch → Changes →
// PR/CI → Announce) is reachable from an issue in the Issues section; the rest
// are direct views.
export const sections = ['issues', 'branch', 'review', 'messaging', 'settings'] as const

export type Section = (typeof sections)[number]

interface UiState {
  section: Section
  setSection: (section: Section) => void
  // The issue whose detail the Issues section shows, by key, or null for none.
  selectedIssue: string | null
  selectIssue: (key: string | null) => void
  // The issue view the stream carries, by name, or null for the server's default.
  view: string | null
  setView: (view: string | null) => void
}

// Client UI state (which section is showing, which issue is selected, which
// view the list is of), shared by the nav rail and the content panes without
// threading props between them.
export const useUiStore = create<UiState>((set) => ({
  section: 'issues',
  setSection: (section) => {
    set({ section })
  },
  selectedIssue: null,
  selectIssue: (key) => {
    set({ selectedIssue: key })
  },
  view: null,
  setView: (view) => {
    set({ view })
  },
}))
