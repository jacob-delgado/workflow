import { useId, useRef } from 'react'
import type { KeyAction } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { boundActions, helpAction } from './boundActions.ts'
import { pressable } from './keyNames.ts'
import { useKeysStore } from './keysApi.ts'
import { ModalDialog } from './ModalDialog.tsx'

interface ShortcutSheetProps {
  // helpKey is the key the sheet opens on, as the page answers it.
  helpKey: string
  onClose: () => void
  onSettings: () => void
}

// A help group as the sheet lists it: the terminal's group name and the
// actions in it this page binds, in the terminal's order.
interface Group {
  name: string
  actions: KeyAction[]
}

// boundGroups is the actions this page binds, under the terminal's groups, in
// the order its help lists them.
function boundGroups(actions: KeyAction[]): Group[] {
  const groups: Group[] = []

  for (const action of actions.filter((listed) => boundActions.has(listed.action))) {
    const last = groups.at(-1)
    if (last?.name === action.group) {
      last.actions.push(action)
    } else {
      groups.push({ name: action.group, actions: [action] })
    }
  }

  return groups
}

// ShortcutSheet is the page's ?: every action this page binds, by the
// terminal's groups and in its words, with the key that triggers it here, and
// whether single keys are on.
export function ShortcutSheet({ helpKey, onClose, onSettings }: ShortcutSheetProps) {
  const headingId = useId()
  const close = useRef<HTMLButtonElement>(null)
  const { shortcuts, actions } = useKeysStore()

  return (
    <ModalDialog
      namedBy={headingId}
      takesFocus={close}
      onClose={onClose}
      className="m-auto max-h-[85dvh] w-[60rem] overflow-y-auto"
    >
      <div className="flex flex-col gap-block p-block">
        <div className="flex items-start justify-between gap-group">
          <div className="flex flex-col gap-tight">
            <h2 id={headingId} className="text-lg">
              Keyboard shortcuts
            </h2>
            <ShortcutState shortcuts={shortcuts} onSettings={onSettings} />
          </div>
          <Button ref={close} variant="secondary" size="sm" onClick={onClose}>
            Close
          </Button>
        </div>
        <div className="gap-block sm:columns-2 lg:columns-3">
          {boundGroups(actions).map((group) => (
            <GroupTable key={group.name} group={group} helpKey={helpKey} />
          ))}
        </div>
      </div>
    </ModalDialog>
  )
}

// ShortcutState says when a single key acts, or that none does until the
// setting is on, with the way to turn it on.
function ShortcutState({ shortcuts, onSettings }: { shortcuts: boolean; onSettings: () => void }) {
  if (shortcuts) {
    return (
      <p className="text-sm text-muted-foreground">
        Each key works outside a text field. Ctrl+K or ⌘K finds an action by name.
      </p>
    )
  }

  return (
    <p className="text-sm text-muted-foreground">
      Single-key shortcuts are off, so only ? and the palette, on Ctrl+K or ⌘K, work.{' '}
      <button
        type="button"
        onClick={onSettings}
        className="rounded-sm text-primary underline underline-offset-2 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        Turn them on in Settings
      </button>
    </p>
  )
}

// GroupTable is one group's actions: each key beside the terminal's words for
// it. An action on a key the page cannot bind — a combination the browser
// keeps — is reached from the palette instead, and says so.
function GroupTable({ group, helpKey }: { group: Group; helpKey: string }) {
  return (
    <table className="mb-block w-full break-inside-avoid text-sm">
      <caption className="pb-tight text-left font-semibold">{group.name}</caption>
      <tbody>
        {group.actions.map((action) => (
          <tr key={action.action}>
            <th scope="row" className="w-16 py-0.5 pr-3 text-left align-top font-normal">
              <KeyCell action={action} helpKey={helpKey} />
            </th>
            <td className="py-0.5">{action.help}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

// KeyCell is the key an action answers here: the sheet's own key for the
// sheet, the help's range for the pane numbers, and otherwise the key shown,
// or the palette where that key is not one the page binds.
function KeyCell({ action, helpKey }: { action: KeyAction; helpKey: string }) {
  if (action.action === helpAction) {
    return <Key>{helpKey}</Key>
  }

  if (!action.keys.some(pressable)) {
    return <span className="text-xs text-muted-foreground">palette</span>
  }

  return <Key>{action.shown}</Key>
}

// Key is a key as it is pressed.
export function Key({ children }: { children: string }) {
  return (
    <kbd className="rounded-sm border border-border px-1.5 font-mono text-xs text-foreground">
      {children}
    </kbd>
  )
}
