import { Button } from './Button.tsx'
import type { Teller } from './Outcome.tsx'
import { useAsyncAction } from './useAsyncAction.ts'

// copyAddress puts an address on the clipboard.
function copyAddress(url: string): Promise<void> {
  return navigator.clipboard.writeText(url)
}

interface CopyURLProps {
  url: string
  mark: string
  teller: Teller
}

// CopyURL copies a pull request's URL — the interface's "copy url" — and says
// so through the teller, into the outcome line beside the control; a copy the
// browser refuses says, beside it, how to get the URL. mark is how the forge
// writes the pull request, #128 or !128.
export function CopyURL({ url, mark, teller }: CopyURLProps) {
  const copy = useAsyncAction(copyAddress, {
    fallback: `The URL of ${mark} could not be copied; open it, and copy it from the address bar.`,
    done: () => `Copied the URL of ${mark}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <>
      <Button
        variant="secondary"
        onClick={() => {
          void copy.run(url)
        }}
      >
        Copy URL <span className="sr-only">to {mark}</span>
      </Button>
      {copy.state === 'error' ? (
        <p role="alert" className="basis-full text-destructive">
          {copy.error}
        </p>
      ) : null}
    </>
  )
}
