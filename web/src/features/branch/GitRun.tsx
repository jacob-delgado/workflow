import { useEffect, useRef, useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Run, RunEvent, RunRequest } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { Failure } from '@/lib/Status.tsx'
import { refusalOf, useAsyncAction } from '@/lib/useAsyncAction.ts'
import { startRun, stopRun } from './gitRunApi.ts'

// runTitle is how the terminal titles a run, for the moment before the
// server's own word lands: a hook by its name, any other by its git command.
function runTitle(request: RunRequest): string {
  switch (request.kind) {
    case 'hook':
      return request.hook ?? 'a hook'
    case 'rebase':
      return 'git rebase'
    case 'amend':
      return 'git commit --amend'
    case 'fixup':
      return 'git commit --fixup'
  }
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

// OwnRun is where the run this page started stands: none, or put away;
// unrun, refused before it began or cut off after, with the words saying so;
// going; or ended, with the write that numbered its end when an end event
// carried one.
type OwnRun =
  | { at: 'none' }
  | { at: 'unrun'; words: string }
  | { at: 'going'; run: Run }
  | { at: 'ended'; run: Run; write: number | undefined }

// useGitRun starts git runs and follows the one this page started as it
// streams: each line as it is written, then how it ended, which stays until it
// is put away. A run some other page started, which the stream carries while
// it goes, stands in when this page has none, and takes the place of one that
// has ended in a frame read after that end, whose writes count it. A run
// refused before it ran says why in refusal, and a run whose stream broke
// once it began says so there, never that it did not run, until the next
// start or Close. A stop refused says why there too, but only while a run is
// shown: the refusal is that run's, this page's or a streamed one, and goes
// with it.
export function useGitRun(streamed: Run | undefined, writes: number): GitRunner {
  const [own, setOwn] = useState<OwnRun>({ at: 'none' })
  const stopping = useAsyncAction(stopRun, {
    fallback: 'The run could not be stopped. Try again.',
  })
  const putAway = () => {
    setOwn({ at: 'none' })
    stopping.reset()
  }

  // The stream carries a run only while it goes, so a run a frame read after
  // this page's run ended carries is another page's: it puts this page's away,
  // as Close does. A frame read before the end, which a stale or closed
  // stream keeps, may still carry this page's own run going, and puts nothing
  // away. Set while rendering, so the frame that carries it draws it.
  if (endedBefore(own, writes) && streamed !== undefined) {
    putAway()
  }
  const mine = standing(own)
  const shown = mine.run ?? streamed ?? null
  // A stop refused as its run ends, or answered once it was put away, would
  // otherwise say why alone, beside no run. Reset while rendering, as above;
  // only in error, when no stop is in flight for the reset to drop.
  if (shown === null && stopping.state === 'error') {
    stopping.reset()
  }

  // No round guards a start's answer: every opener is held while a run goes,
  // Close is offered only once the end is handed on, and the answer settles
  // in that same tick or before, so no later start or Close can come between.
  const start = (request: RunRequest) => {
    stopping.reset()
    setOwn(started(request))
    let begun: Run | null = null
    const follow = (event: RunEvent) => {
      begun = event.run ?? begun
      setOwn((previous) => followed(previous, event))
    }
    startRun(request, follow).then(
      (last) => {
        setOwn((previous) => answered(previous, last))
      },
      (caught: unknown) => {
        const words = begun === null ? notRun(request, caught) : cutOff(begun)
        setOwn({ at: 'unrun', words })
      },
    )
  }

  return {
    run: shown,
    going: shown?.state === 'in_progress',
    // A start resets the stop, so a stop's refusal is always the newer.
    refusal: refusalOf(stopping) || mine.words,
    start,
    stop: () => void stopping.run(),
    close: putAway,
  }
}

// standing is what this page's own run shows: the run, or why none ran.
function standing(own: OwnRun): { run: Run | null; words: string } {
  switch (own.at) {
    case 'none':
      return { run: null, words: '' }
    case 'unrun':
      return { run: null, words: own.words }
    case 'going':
    case 'ended':
      return { run: own.run, words: '' }
  }
}

// started is a run just asked for, titled as the terminal titles it until the
// server's own word lands.
function started(request: RunRequest): OwnRun {
  return {
    at: 'going',
    run: {
      kind: request.kind,
      title: runTitle(request),
      state: 'in_progress',
      outcome: '',
      lines: [],
    },
  }
}

// notRun says why a run refused before it began did not run.
function notRun(request: RunRequest, caught: unknown): string {
  return apiErrorMessage(
    caught,
    `${runTitle(request)} did not run. Try again, or run it from a terminal.`,
  )
}

// cutOff says a run's stream broke after the run began: it ran, in part or
// to its end, or goes on, and the stream shows it while it does.
function cutOff(begun: Run): string {
  return `${begun.title} stopped answering before it said how it ended. It may have run in part, or still be running; the branch shows where it stands.`
}

// endedBefore reports this page's run ended in a numbered write that a frame
// counting writes was read after.
function endedBefore(own: OwnRun, writes: number): boolean {
  return own.at === 'ended' && own.write !== undefined && writes >= own.write
}

// followed is this page's run after one more of its events: a line added to
// it, or the run as the server says it stands, keeping the lines already
// shown while it is still going, and the write numbering its end once it
// ends. Only a going run hears events; any other stage is kept as it is.
function followed(own: OwnRun, event: RunEvent): OwnRun {
  if (own.at !== 'going') {
    return own
  }

  if (event.line !== undefined) {
    return { at: 'going', run: { ...own.run, lines: [...own.run.lines, event.line] } }
  }

  if (event.run === undefined) {
    return own
  }

  return event.run.state === 'in_progress'
    ? { at: 'going', run: { ...event.run, lines: own.run.lines } }
    : { at: 'ended', run: event.run, write: event.write }
}

// answered is this page's run once its start answers with the last run the
// stream carried, keeping the write an end event already numbered.
function answered(own: OwnRun, last: Run): OwnRun {
  if (last.state === 'in_progress') {
    return { at: 'going', run: last }
  }

  return { at: 'ended', run: last, write: own.at === 'ended' ? own.write : undefined }
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
