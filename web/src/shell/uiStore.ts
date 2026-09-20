import { create } from 'zustand'

// The cockpit's sections, in nav order. The work story (Branch → Changes →
// PR/CI → Announce) is reachable from an issue in the Issues section; the rest
// are direct views.
export const sections = ['issues', 'branch', 'review', 'slack', 'settings'] as const

export type Section = (typeof sections)[number]

interface UiState {
  section: Section
  setSection: (section: Section) => void
}

// Client UI state (which section is showing), shared by the nav rail and the
// content area without threading props between them.
export const useUiStore = create<UiState>((set) => ({
  section: 'issues',
  setSection: (section) => {
    set({ section })
  },
}))
