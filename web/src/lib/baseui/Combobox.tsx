import type { ReactNode } from 'react'
import { Combobox as Base } from '@base-ui/react/combobox'
import { cn } from '@/lib/utils.ts'

interface ComboboxProps<Item> {
  // label names the field, which shows placeholder until something is typed.
  label: string
  placeholder: string
  items: Item[]
  itemKey: (item: Item) => string
  itemText: (item: Item) => string
  // empty is what is said while nothing matches what was typed.
  empty: ReactNode
  // held is a field whose choice is saving: off, by aria-disabled, so it
  // keeps the focus it was chosen with, and taking no typing.
  held?: boolean
  disabled?: boolean
  onChoose: (item: Item) => void
}

// Combobox is a field that offers a list of items to choose one from, which
// typing narrows to those whose words hold what was typed; Up and Down move
// through it, and Enter or a press chooses. The choice is handed on rather
// than kept: the field goes back to empty once one is chosen.
export function Combobox<Item>({
  label,
  placeholder,
  items,
  itemKey,
  itemText,
  empty,
  held = false,
  disabled = false,
  onChoose,
}: ComboboxProps<Item>) {
  return (
    <Base.Root
      items={items}
      value={null}
      itemToStringLabel={itemText}
      disabled={disabled}
      readOnly={held}
      onValueChange={(chosen: Item | null) => {
        if (chosen !== null) {
          onChoose(chosen)
        }
      }}
    >
      <Base.Input
        aria-label={label}
        aria-disabled={held || undefined}
        placeholder={placeholder}
        className={cn(
          'w-48 max-w-full min-w-0 rounded-md border border-input bg-background px-2 py-1 text-sm text-foreground placeholder:text-muted-foreground',
          'focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
          'disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground',
          'aria-disabled:cursor-not-allowed aria-disabled:bg-disabled aria-disabled:text-disabled-foreground',
        )}
      />
      <Base.Portal>
        <Base.Positioner sideOffset={4} className="z-50">
          <Base.Popup className="max-h-64 w-(--anchor-width) min-w-48 overflow-y-auto rounded-md border border-border bg-popover py-1 text-sm text-popover-foreground">
            <Base.Empty className="px-3 py-1.5 text-muted-foreground empty:hidden">
              {empty}
            </Base.Empty>
            <Base.List>
              {(item: Item) => (
                <Base.Item
                  key={itemKey(item)}
                  value={item}
                  className="cursor-pointer px-3 py-1.5 hover:bg-accent/60 data-highlighted:bg-accent"
                >
                  {itemText(item)}
                </Base.Item>
              )}
            </Base.List>
          </Base.Popup>
        </Base.Positioner>
      </Base.Portal>
    </Base.Root>
  )
}
