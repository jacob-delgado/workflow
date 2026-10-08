import {
  getHookSetup,
  setUpHooks,
  startRun as postRun,
  stopRun as deleteRun,
} from '@/api/generated'
import type {
  HookSetup,
  HookSetupWritten,
  Run,
  RunEvent,
  RunRequest,
} from '@/api/generated/types.gen.ts'
import { zRunEvent } from '@/api/generated/zod.gen.ts'

// startRun starts a git run — pre-commit, a rebase, an amend or a fixup — and
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
// answers the last run it carried.
async function readEvents(
  body: ReadableStream<Uint8Array>,
  onEvent: (event: RunEvent) => void,
): Promise<Run> {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let last: Run | undefined

  for (;;) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''
    for (const line of lines) {
      const event = parsed(line)
      if (event !== null) {
        last = event.run ?? last
        onEvent(event)
      }
    }
  }

  if (last === undefined) {
    throw new Error('the run ended without saying how')
  }

  return last
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
