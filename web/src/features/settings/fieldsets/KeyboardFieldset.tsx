import { useState } from 'react'
import type { KeyAction } from '@/api/generated/types.gen.ts'
import { useKeysStore } from '@/features/keyboard/keysApi.ts'
import { Input } from '@/lib/Field.tsx'
import { CheckboxField, Fieldset, type Register } from './Field.tsx'

// notRebindable is the one action ui.keys cannot move: its keys are the pane
// numbers, which no one key can stand in for, and the interface refuses a map
// that names it.
const notRebindable = 'jump-to-pane'

// KeyboardFieldset is whether a single key, pressed outside a text field, does
// what the terminal's does — off until turned on, since a key that acts on its
// own surprises a screen reader or speech user; ? and the palette work either
// way — and, behind Rebind keys, the key each action is on (ui.keys).
export function KeyboardFieldset({ register }: { register: Register }) {
  return (
    <Fieldset
      legend="Keyboard"
      hint="A key pressed outside a text field does what it does in the terminal, as the keys below set it. The ? sheet of keys and the palette, on Ctrl+K or ⌘K, work whether this is on or off."
    >
      <CheckboxField register={register} name="ui.web_shortcuts" label="Single-key shortcuts" />
      <KeyTable register={register} />
    </Fieldset>
  )
}

// groupsOf is the actions that can be rebound, in the help's groups and order.
function groupsOf(actions: KeyAction[]): [group: string, actions: KeyAction[]][] {
  const groups = new Map<string, KeyAction[]>()
  for (const action of actions) {
    if (action.action !== notRebindable) {
      groups.set(action.group, [...(groups.get(action.group) ?? []), action])
    }
  }

  return [...groups]
}

// KeyTable is every action the terminal binds, grouped as its help groups
// them, each with the key it is moved to, empty for its default. It is folded
// away until opened, and drawn only then: most never move a key, and a folded
// field would still sit in the Tab order. A key moved and folded away again
// stays moved, since the form keeps what it was given. A key that clashes with another
// where both are live is refused when the form is saved, as the terminal
// refuses it as it starts.
function KeyTable({ register }: { register: Register }) {
  const actions = useKeysStore((state) => state.actions)
  const [open, setOpen] = useState(false)
  if (actions.length === 0) {
    return null
  }

  return (
    <details
      className="text-sm"
      onToggle={(event) => {
        setOpen(event.currentTarget.open)
      }}
    >
      <summary className="cursor-pointer text-muted-foreground">Rebind keys</summary>
      {open ? <KeyGroups register={register} actions={actions} /> : null}
    </details>
  )
}

// KeyGroups is the table of keys, a group of rows per help group.
function KeyGroups({ register, actions }: { register: Register; actions: KeyAction[] }) {
  return (
    <table aria-label="Keys" className="mt-item w-full table-fixed text-left">
      <thead className="text-xs text-muted-foreground">
        <tr>
          <th scope="col" className="py-1 font-medium">
            Action
          </th>
          <th scope="col" className="w-40 py-1 font-medium">
            Key
          </th>
        </tr>
      </thead>
      {groupsOf(actions).map(([group, listed]) => (
        <tbody key={group}>
          <tr>
            <th scope="rowgroup" colSpan={2} className="pt-group pb-1 font-semibold">
              {group}
            </th>
          </tr>
          {listed.map((action) => (
            <KeyRow key={action.action} register={register} action={action} />
          ))}
        </tbody>
      ))}
    </table>
  )
}

// KeyRow is one action: what it does and its name in ui.keys, and the key it
// is moved to, described by the default an empty one keeps.
function KeyRow({ register, action }: { register: Register; action: KeyAction }) {
  const hintId = `ui.keys.${action.action}-hint`

  return (
    <tr className="border-t border-border align-top">
      <th scope="row" className="py-1 pr-item font-normal break-words">
        {action.help}
        <span className="block font-mono text-xs text-muted-foreground">{action.action}</span>
      </th>
      <td className="py-1">
        <Input
          size="sm"
          className="w-full"
          aria-label={`Key for ${action.action}`}
          aria-describedby={hintId}
          {...register(`ui.keys.${action.action}`)}
        />
        <p id={hintId} className="text-xs text-muted-foreground">
          Empty keeps {action.default}.
        </p>
      </td>
    </tr>
  )
}
