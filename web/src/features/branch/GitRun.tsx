import { useEffect, useRef, useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Run, RunEvent, RunRequest } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { Failure } from '@/lib/Status.tsx'
import { startRun, stopRun } from './gitRunApi.ts'

// runTitle is how the terminal titles each run, for the moment before the
// server's own word lands.
const runTitle: Record<RunRequest['kind'], string> = {
  pre_commit: 'pre-commit',
  rebase: 'git rebase',
  amend: 'git commit --amend',
  fixup: 'git commit --fixup',
}

// GitRunner is the Branch section's one git run: the run this page started,
// or the one the stream says is going, and how to start, stop and put away
// a run.
export interface GitRunner {
  run: Run | null
  going: boolean
  refusal: string
  start: (request: RunRequest) => void
  stop: () => void
  close: () => void
}

// useGitRun starts git runs and follows the one this page started as it
// streams: each line as it is written, then how it ended, which stays until it
// is put away. A run some other page started, which the stream carries while
// it goes, stands in when this page has none. A run refused before it ran
// says why in refusal.
export function useGitRun(streamed: Run | undefined): GitRunner {
  const [run, setRun] = useState<Run | null>(null)
  const [refusal, setRefusal] = useState('')
  const shown = run ?? streamed ?? null

  const follow = (event: RunEvent) => {
    setRun((previous) => followed(previous, event))
  }

  const start = (request: RunRequest) => {
    setRefusal('')
    setRun({
      kind: request.kind,
      title: runTitle[request.kind],
      state: 'in_progress',
      outcome: '',
      lines: [],
    })
    startRun(request, follow).then(setRun, (caught: unknown) => {
      setRun(null)
      setRefusal(
        apiErrorMessage(
          caught,
          `${runTitle[request.kind]} did not run. Try again, or run it from a terminal.`,
        ),
      )
    })
  }

  const stop = () => {
    stopRun().catch((caught: unknown) => {
      setRefusal(apiErrorMessage(caught, 'The run could not be stopped. Try again.'))
    })
  }

  return {
    run: shown,
    going: shown?.state === 'in_progress',
    refusal,
    start,
    stop,
    close: () => {
      setRun(null)
    },
  }
}

// followed is the run after one more of its events: a line added to it, or
// the run as the server says it stands, keeping the lines already shown while
// it is still going.
function followed(previous: Run | null, event: RunEvent): Run | null {
  if (event.line !== undefined && previous !== null) {
    return { ...previous, lines: [...previous.lines, event.line] }
  }

  if (event.run === undefined) {
    return previous
  }

  return event.run.state === 'in_progress'
    ? { ...event.run, lines: previous?.lines ?? [] }
    : event.run
}

// RunOutput is a git run: what it writes, as it writes it, kept scrolled to
// its newest line; Stop while it goes, and once it has ended, how it ended —
// a refusal said as one — and Close to put it away.
export function RunOutput({ runner }: { runner: GitRunner }) {
  const { run } = runner
  const output = useRef<HTMLPreElement>(null)
  const lines = run?.lines.length ?? 0

  useEffect(() => {
    if (output.current !== null) {
      output.current.scrollTop = output.current.scrollHeight
    }
  }, [lines])

  if (run === null) {
    return runner.refusal === '' ? null : <Failure>{runner.refusal}</Failure>
  }

  return (
    <section
      aria-label={`Run of ${run.title}`}
      className="flex flex-col gap-item rounded-lg border border-border p-4"
    >
      <p role="status" className="text-sm font-medium">
        {run.state === 'in_progress' ? `Running ${run.title}…` : endedWords(run)}
      </p>
      <pre
        ref={output}
        role="region"
        aria-label={`Output of ${run.title}`}
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- output that scrolls must be reachable by Tab to be scrolled by keys (WCAG 2.1.1)
        tabIndex={0}
        className="max-h-80 overflow-auto rounded-md border border-border p-3 text-xs [overflow-wrap:anywhere] whitespace-pre-wrap focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        {run.lines.length === 0 ? 'Nothing written yet.' : run.lines.join('\n')}
      </pre>
      {run.state === 'refused' ? <Failure>{run.outcome}</Failure> : null}
      {runner.refusal === '' ? null : <Failure>{runner.refusal}</Failure>}
      {run.state === 'in_progress' ? (
        <Button
          variant="secondary"
          aria-label={`Stop ${run.title}`}
          onClick={runner.stop}
          className="self-start"
        >
          Stop
        </Button>
      ) : (
        <Button variant="secondary" onClick={runner.close} className="self-start">
          Close
        </Button>
      )}
    </section>
  )
}

// endedWords says how a run ended that is not a refusal, which says itself.
function endedWords(run: Run): string {
  return run.state === 'refused' ? `${run.title} was refused.` : run.outcome
}
