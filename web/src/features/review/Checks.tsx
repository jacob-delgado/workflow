import type { Check, Ci, JobLog } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { Meta } from '@/lib/Meta.tsx'
import { NewTabLink } from '@/lib/NewTabLink.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { ciMark, StateMark } from '@/shell/StateMark.tsx'
import { readCheckLog } from './checkLogApi.ts'

// CiChecks is how the pull request's CI stands, headed by how many checks are
// done when the forge counts them and by the state alone when it does not — a
// GitLab pipeline, or checks not started — as the terminal's ciSummary words
// it; with no checks at all it says none are reported rather than draw an empty
// list.
export function CiChecks({ ci }: { ci: Ci }) {
  const standing = ciStanding(ci)

  return (
    <section aria-labelledby="ci-heading" className="flex flex-col gap-group">
      <h3 id="ci-heading" className="text-base font-semibold">
        <Meta>
          CI checks
          {standing === null ? null : (
            <span className="font-normal text-muted-foreground">{standing}</span>
          )}
        </Meta>
      </h3>
      {ci.checks.length === 0 ? (
        <p className="flex items-center gap-item text-sm text-muted-foreground">
          <StateMark state="unknown" />
          No checks reported.
        </p>
      ) : (
        <ul className="flex flex-col gap-item">
          {ci.checks.map((check) => (
            <CheckRow key={check.id ?? check.name} check={check} />
          ))}
        </ul>
      )}
    </section>
  )
}

// ciStanding is the heading's second fact: the count when the forge gives one,
// the state word when it does not, and none for a state of none, which the
// line beneath the heading says.
function ciStanding(ci: Ci): string | null {
  if (ci.total > 0) {
    const failed = ci.failed > 0 ? `, ${String(ci.failed)} failed` : ''

    return `${String(ci.done)} of ${String(ci.total)} done${failed}`
  }

  return ci.state === 'none' ? null : ci.state
}

// CheckRow is one check: how it stands, the stage it ran in for a GitLab job,
// then its name — unique in its pipeline — linked to its page, and for a
// failed one, why it failed.
function CheckRow({ check }: { check: Check }) {
  return (
    <li className="flex flex-col gap-1 text-sm">
      <span className="flex items-center gap-item">
        <StateMark state={ciMark[check.state]} />
        <Meta>
          {check.stage ? <span className="text-muted-foreground">{check.stage}</span> : null}
          {check.url === '' ? check.name : <NewTabLink href={check.url}>{check.name}</NewTabLink>}
        </Meta>
        <span className="text-muted-foreground">{check.state}</span>
      </span>
      {check.state === 'failed' && check.reason ? (
        <span className="pl-6 text-muted-foreground">{check.reason}</span>
      ) : null}
      {check.log_available && check.id ? <CheckLog id={check.id} name={check.name} /> : null}
    </li>
  )
}

// CheckLog reads a check's log when asked, never before: a log can be
// long, and the forge counts every read. The log takes the place, and the
// focus, of the control that asked for it, and scrolls by the keyboard.
function CheckLog({ id, name }: { id: string; name: string }) {
  const read = useAsyncAction(readCheckLog, { fallback: 'The log could not be read. Try again.' })

  if (read.state === 'done' && read.result) {
    return <LogText name={name} log={read.result} />
  }

  return (
    <span className="flex flex-col gap-1 pl-6">
      <Button
        variant="secondary"
        className="self-start"
        aria-label={
          read.state === 'running' ? `Reading the log of ${name}…` : `Show log of ${name}`
        }
        aria-disabled={read.state === 'running'}
        onClick={() => {
          if (read.state !== 'running') {
            void read.run(id)
          }
        }}
      >
        {read.state === 'running' ? 'Reading the log…' : 'Show log'}
      </Button>
      {read.state === 'error' ? (
        <span role="alert" className="text-destructive">
          {read.error}
        </span>
      ) : null}
    </span>
  )
}

function LogText({ name, log }: { name: string; log: JobLog }) {
  const region = useFocusOnMount<HTMLPreElement>()

  return (
    <pre
      ref={region}
      role="region"
      aria-label={`Log of ${name}`}
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- a log that scrolls must be reachable by Tab to be scrolled by keys (WCAG 2.1.1)
      tabIndex={0}
      className="ml-6 max-h-80 overflow-auto rounded-md border border-border p-3 text-xs whitespace-pre-wrap focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      {log.truncated ? '… earlier lines are not shown\n' : ''}
      {log.text}
    </pre>
  )
}
