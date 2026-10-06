import type { HookSetup as Offer } from '@/api/generated/types.gen.ts'
import { useHoldShortcuts, useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import type { Teller } from '@/lib/Outcome.tsx'
import { Failure, Reading } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { readHookSetup, writeHookSetup } from './gitRunApi.ts'

// HookSetup offers a lefthook configuration for the hooks the repository runs
// that lefthook does not manage, as the terminal's g does, while the stream
// counts any. Opening it reads what would be written, which is its own
// preview; Write lefthook.yml writes it and installs lefthook.
export function HookSetup({ unmanaged, outcome }: { unmanaged: number; outcome: Teller }) {
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const setUpKeys = useShortcut('set-up-lefthook', opener)
  const read = useAsyncAction(readHookSetup, {
    fallback: 'The hooks could not be read. Try again, or set lefthook up with g in the terminal.',
  })

  if (unmanaged === 0) {
    return null
  }

  if (read.state === 'done' && read.result !== undefined) {
    return (
      <SetupLook
        offer={read.result}
        outcome={outcome}
        onClose={() => {
          handBack()
          read.reset()
        }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="secondary"
        ref={opener}
        aria-keyshortcuts={setUpKeys}
        aria-disabled={read.state === 'running'}
        onClick={() => {
          if (read.state !== 'running') {
            void read.run()
          }
        }}
        className="self-start"
      >
        Set up lefthook
      </Button>
      {read.state === 'running' ? <Reading>Reading the hooks…</Reading> : null}
      {read.state === 'error' ? <Failure>{read.error}</Failure> : null}
    </div>
  )
}

// SetupLook is the offer: the hooks found, the lefthook.yml that runs them,
// and the two ways to write it, or neither.
function SetupLook({
  offer,
  outcome,
  onClose,
}: {
  offer: Offer
  outcome: Teller
  onClose: () => void
}) {
  const shown = useFocusOnMount<HTMLElement>()
  useHoldShortcuts()
  const write = useAsyncAction(writeHookSetup, {
    fallback: 'lefthook.yml was not written. Try again, or set it up with g in the terminal.',
    done: (written) =>
      written.scripts === 0
        ? 'Wrote lefthook.yml and installed lefthook.'
        : `Wrote lefthook.yml and ${String(written.scripts)} script(s), and installed lefthook.`,
    onStart: outcome.clear,
    onDone: (said) => {
      outcome.say(said)
    },
  })
  const writing = write.state === 'running'

  if (write.state === 'done') {
    return null
  }

  if (!offer.offered) {
    return (
      <p className="text-sm text-muted-foreground">
        Nothing to set up: lefthook is configured, or no hook lacks it.
      </p>
    )
  }

  return (
    <section
      ref={shown}
      tabIndex={-1}
      aria-label="Set up lefthook"
      className="flex flex-col gap-group rounded-lg border border-border p-4"
    >
      <p className="text-sm">
        Found {offer.hooks.length} hook(s) in .git/hooks that lefthook does not manage:{' '}
        {offer.hooks.map((hook) => `${hook.name} (${String(hook.lines)} lines)`).join(', ')}.
      </p>
      <pre
        role="region"
        aria-label="lefthook.yml that runs them"
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- a file that scrolls must be reachable by Tab to be scrolled by keys (WCAG 2.1.1)
        tabIndex={0}
        className="max-h-60 overflow-auto rounded-md border border-border p-3 text-xs [overflow-wrap:anywhere] whitespace-pre-wrap focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        {offer.config}
      </pre>
      <p className="text-xs text-muted-foreground">
        {offer.scripts > 0
          ? `${String(offer.scripts)} hook(s) kept whole as scripts under .lefthook. `
          : ''}
        lefthook install then keeps the old hooks as .git/hooks/*.old.
      </p>
      {write.state === 'error' ? <Failure>{write.error}</Failure> : null}
      <SetupButtons
        writing={writing}
        onClose={onClose}
        onWrite={(verbatim) => {
          void write.run(verbatim)
        }}
      />
    </section>
  )
}

interface SetupButtonsProps {
  writing: boolean
  onClose: () => void
  onWrite: (verbatim: boolean) => void
}

// SetupButtons are the offer's choices: neither, every hook kept whole as a
// script, or the configuration shown.
function SetupButtons({ writing, onClose, onWrite }: SetupButtonsProps) {
  return (
    <div className="flex flex-wrap items-center gap-item">
      <Button variant="secondary" disabled={writing} onClick={onClose}>
        Cancel
      </Button>
      <Button
        variant="secondary"
        disabled={writing}
        onClick={() => {
          onWrite(true)
        }}
      >
        Write every hook as a script
      </Button>
      <Button
        variant="primary"
        disabled={writing}
        onClick={() => {
          onWrite(false)
        }}
      >
        {writing ? 'Writing…' : 'Write lefthook.yml'}
      </Button>
    </div>
  )
}
