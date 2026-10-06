import { useId, useState } from 'react'
import type { TaskList } from '@/api/generated/types.gen.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { Input } from '@/lib/Field.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'

interface TaskLineFormProps {
  // shortcut is the terminal's action that puts the focus in the line.
  shortcut: string
  // command is what the line follows in Taskwarrior's grammar — "task add",
  // "task 12 modify" — which names the field.
  command: string
  // verb names the button that sends the line, and busy says it is sending.
  verb: string
  busy: string
  send: (line: string) => Promise<TaskList>
  // done is what the panel's outcome line says once Taskwarrior has taken it.
  done: (answered: TaskList) => string
  fallback: string
  teller: Teller
}

// TaskLineForm is one line in Taskwarrior's own grammar — words, and attributes
// such as project:web, due:friday or +tag among them — sent after its command:
// an add, a note or a modify. An empty line is refused where it is typed, as
// Taskwarrior would refuse it; a line Taskwarrior refuses stays in the field
// beside its reason, and one it takes is cleared.
export function TaskLineForm({
  shortcut,
  command,
  verb,
  busy,
  send,
  done,
  fallback,
  teller,
}: TaskLineFormProps) {
  const field = useId()
  const input = useShortcutProps<HTMLInputElement>(shortcut, 'focus')
  const [line, setLine] = useState('')
  const [empty, setEmpty] = useState(false)
  const write = useAsyncAction(
    async (typed: string) => {
      const answered = await send(typed)
      setLine('')

      return answered
    },
    { fallback, done, onStart: teller.clear, onDone: teller.say },
  )
  const refusal = refusalOf(empty, command, write.state === 'error' ? write.error : '')

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        const typed = line.trim()
        setEmpty(typed === '')
        if (typed !== '') {
          void write.run(typed)
        }
      }}
      className="flex flex-wrap items-center gap-item"
    >
      <label htmlFor={field} className="font-mono text-sm text-muted-foreground">
        {command}
      </label>
      <Input
        {...input}
        id={field}
        value={line}
        onChange={(event) => {
          setLine(event.target.value)
          setEmpty(false)
        }}
        className="min-w-40 flex-1"
      />
      <Button variant="secondary" type="submit" disabled={write.state === 'running'}>
        {write.state === 'running' ? busy : verb}
      </Button>
      {refusal === '' ? null : (
        <p role="alert" className="basis-full text-sm whitespace-pre-line text-destructive">
          {refusal}
        </p>
      )}
    </form>
  )
}

// refusalOf is why the line was not sent, or was refused: empty, before it
// left, or Taskwarrior's reason — '' when neither stands.
function refusalOf(empty: boolean, command: string, refused: string): string {
  return empty ? `Type a line for ${command} first.` : refused
}
