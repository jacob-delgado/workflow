import type { Check, JobLog } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { Meta } from '@/lib/Meta.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { ciMark, StateMark } from '@/shell/StateMark.tsx'
import { readCheckLog } from './checkLogApi.ts'

// CheckRow is one check: how it stands, the stage it ran in for a GitLab job,
// then its name — unique in its pipeline — linked to its page, and for a
// failed one, why it failed.
export function CheckRow({ check }: { check: Check }) {
  return (
    <li className="flex flex-col gap-1 text-sm">
      <span className="flex items-center gap-item">
        <StateMark state={ciMark[check.state]} />
        <Meta>
          {check.stage ? <span className="text-muted-foreground">{check.stage}</span> : null}
          {check.url === '' ? (
            check.name
          ) : (
            <a
              href={check.url}
              target="_blank"
              rel="noreferrer"
              className="underline-offset-4 hover:underline"
            >
              {check.name}
            </a>
          )}
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
