import { useId } from 'react'
import { useHealthStore } from '@/api/health.ts'
import { PeopleTable } from './PeopleTable.tsx'
import { RepoGroups } from './RepoGroups.tsx'

// PeopleAndGroups is Settings' People and groups area: whom each code owner
// is on Slack, and the user groups this repository's announcements may tag.
// Each saves on its own, apart from the configuration, into the kept file.
export function PeopleAndGroups() {
  const headingId = useId()
  const dryRun = useHealthStore((state) => state.health?.dry_run === true)

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-group">
      <h2 id={headingId} className="text-base font-semibold">
        People and groups
      </h2>
      <p className="text-xs text-muted-foreground">
        Whom each code owner is on Slack, and the user groups an announcement may tag, kept on this
        machine. Each change saves on its own, apart from the configuration.
      </p>
      {dryRun ? (
        <p className="text-sm text-muted-foreground">
          Changes are held back: workflow was started with{' '}
          <code className="font-mono">--dry-run</code>, so nothing here is saved.
        </p>
      ) : null}
      <PeopleTable dryRun={dryRun} />
      <RepoGroups dryRun={dryRun} />
    </section>
  )
}
