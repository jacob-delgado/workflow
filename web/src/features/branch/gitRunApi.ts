import {
  getHookSetup,
  listHooks,
  setUpHooks,
  startRun as postRun,
  stopRun as deleteRun,
} from '@/api/generated'
import type {
  ConfiguredHooks,
  HookSetup,
  HookSetupWritten,
  Run,
  RunEvent,
  RunRequest,
} from '@/api/generated/types.gen.ts'
import { zRunEvent } from '@/api/generated/zod.gen.ts'
import { nextFrame } from '@/api/snapshot.ts'

// startRun starts a git run — a hook, a rebase, an amend or a fixup — and
// hands each event it streams to onEvent as it lands: the run as it starts,
// each line of output, the run as it ended, which it answers. A refusal before
// anything ran throws the API error, whose message is safe to show; an event
// that does not match the contract is dropped.
export async function startRun(
  request: RunRequest,
  onEvent: (event: RunEvent) => void,
): Promise<Run> {
  const result = await postRun({ body: request, parseAs: 'stream', throwOnError: true })
  const body: unknown = result.data
  if (!(body instanceof ReadableStream)) {
    throw new Error('the run sent no stream')
  }

  return readEvents(body as ReadableStream<Uint8Array>, onEvent)
}

// readEvents reads a run's stream, one JSON event a line, to its end, and
// answers the last run it carried. How the run ended is handed on only once
// the event stream has pushed the frame that end woke — the server wakes the
// stream before it sends the end — so the run says how it ended beside a
// branch that shows it.
async function readEvents(
  body: ReadableStream<Uint8Array>,
  onEvent: (event: RunEvent) => void,
): Promise<Run> {
  let last: Run | undefined
  let ending: RunEvent | undefined
  let shown: Promise<void> = Promise.resolve()

  for await (const line of linesOf(body)) {
    const event = parsed(line)
    if (event === null) {
      continue
    }

    last = event.run ?? last
    if (event.run !== undefined && event.run.state !== 'in_progress') {
      ending = event
      shown = nextFrame()
    } else {
      onEvent(event)
    }
  }

  if (last === undefined) {
    throw new Error('the run ended without saying how')
  }

  await shown
  if (ending !== undefined) {
    onEvent(ending)
  }

  return last
}

// linesOf is each whole line of a streamed body as it arrives.
async function* linesOf(body: ReadableStream<Uint8Array>): AsyncGenerator<string> {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  for (;;) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''
    yield* lines
  }
}

// parsed is one line of the stream as an event, or null when it is blank or
// does not match the contract.
function parsed(line: string): RunEvent | null {
  if (line.trim() === '') {
    return null
  }

  try {
    const event = zRunEvent.safeParse(JSON.parse(line))

    return event.success ? event.data : null
  } catch {
    return null
  }
}

// stopRun stops the run going, its program and all it started. A refusal —
// none is going — throws the API error.
export async function stopRun(): Promise<void> {
  await deleteRun({ throwOnError: true })
}

// readHooks reads the hooks lefthook configures: those a run by hand can run,
// and those only git runs. A refusal throws the API error.
export async function readHooks(): Promise<ConfiguredHooks> {
  const result = await listHooks({ throwOnError: true })

  return result.data
}

// readHookSetup reads the lefthook configuration offered for the hooks
// lefthook does not manage. A refusal throws the API error.
export async function readHookSetup(): Promise<HookSetup> {
  const result = await getHookSetup({ throwOnError: true })

  return result.data
}

// writeHookSetup writes the offered configuration — or, verbatim, one keeping
// every hook whole as a script — and installs lefthook. A refusal throws the
// API error.
export async function writeHookSetup(verbatim: boolean): Promise<HookSetupWritten> {
  const result = await setUpHooks({ body: { verbatim }, throwOnError: true })

  return result.data
}
