import { useState } from 'react'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import type { Teller } from '@/lib/Outcome.tsx'
import { Failure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { LastLook } from '@/lib/LastLook.tsx'
import { type Removal, removalConsequence, removalWords, useRemovals } from '../removal.ts'

interface CredentialRemovalProps {
  removal: Removal
  // tell is the outcome line of whatever outlives this control: the field, or
  // the list, whose row a removal takes away.
  tell: Teller
}

// CredentialRemoval is Remove… beside a credential the file holds, which asks
// first — a removal writes the file at once and cannot be undone — and then
// writes the file as it was read without it, leaving the form's other edits
// as they are. What it did is said on the outcome line it is handed.
export function CredentialRemoval({ removal, tell }: CredentialRemovalProps) {
  const { holds, remove } = useRemovals()
  const [asking, setAsking] = useState(false)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const words = removalWords(removal)
  const run = useAsyncAction(() => remove(removal), {
    fallback: `${words} was not removed. Try again.`,
    done: () => `Removed ${words}.`,
    onStart: tell.clear,
    onDone: tell.say,
  })

  if (!holds(removal)) {
    return null
  }

  if (asking) {
    return (
      <RemoveQuestion
        removal={removal}
        onCancel={() => {
          setAsking(false)
          handBack()
        }}
        onRemove={() => {
          setAsking(false)
          handBack()
          void run.run()
        }}
      />
    )
  }

  return (
    <>
      <Button
        ref={opener}
        variant="secondary"
        size="sm"
        held={run.state === 'running'}
        aria-label={`Remove ${words}…`}
        onClick={() => {
          setAsking(true)
        }}
      >
        {run.state === 'running' ? 'Removing…' : 'Remove…'}
      </Button>
      {run.state === 'error' ? (
        <div className="basis-full">
          <Failure>{run.error}</Failure>
        </div>
      ) : null}
    </>
  )
}

interface RemoveQuestionProps {
  removal: Removal
  onCancel: () => void
  onRemove: () => void
}

// RemoveQuestion asks before a removal.
function RemoveQuestion({ removal, onCancel, onRemove }: RemoveQuestionProps) {
  return (
    <LastLook
      question={`Remove ${removalWords(removal)} from the file?`}
      cost={
        <>
          {removalConsequence(removal)}It cannot be undone: the file keeps no copy, and the save is
          made at once, without the form&apos;s other changes.
        </>
      }
      act="Remove"
      className="basis-full"
      onAct={onRemove}
      onCancel={onCancel}
    />
  )
}
