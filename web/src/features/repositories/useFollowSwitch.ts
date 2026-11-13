import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { isConfigQuery, readConfigAgain } from '@/features/settings/configApi.ts'
import { useUiStore } from '@/shell/uiStore.ts'

// useFollowSwitch reads every section again once the stream says the server
// works somewhere else — switched from this page, another or the terminal's
// interface — each from nothing but the configuration, read again in place so
// an open Settings form keeps its edits while it reads; and it lets go of the
// issue shown unless it is a Jira issue, which names the same issue wherever
// the server works, where a forge issue's number names one in the repository
// left. What you chose for the session — the section, the views' orders and
// narrowing, the Summary's period — stays.
export function useFollowSwitch(): void {
  const client = useQueryClient()
  const here = useSnapshotStore((state) => state.snapshot?.here)
  const seen = useRef(here)

  useEffect(() => {
    if (here === undefined || here === seen.current) {
      return
    }

    const before = seen.current
    seen.current = here
    if (before === undefined) {
      return
    }

    void client.resetQueries({ predicate: (query) => !isConfigQuery(query) })
    void readConfigAgain(client)
    const selected = useUiStore.getState().selectedIssue
    if (selected !== null && selected.tracker !== 'jira') {
      useUiStore.getState().selectIssue(null)
    }
  }, [client, here])
}
