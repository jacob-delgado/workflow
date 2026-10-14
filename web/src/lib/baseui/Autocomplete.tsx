import type { KeyboardEvent, ReactNode, RefObject } from 'react'
import { Autocomplete as Base } from '@base-ui/react/autocomplete'
import { cn } from '@/lib/utils.ts'

// AutocompleteGroup is what an autocomplete lists under one name. It is a
// type rather than an interface so that it is open to Base UI's group, which
// takes any other key beside items.
export type AutocompleteGroup<Item> = {
  name: string
  items: Item[]
}

interface AutocompleteProps<Item> {
  // groups is what matches what was typed, in the order it is drawn: the
  // caller filters and ranks, and the list draws what it is given.
  groups: AutocompleteGroup<Item>[]
  typed: string
  onType: (typed: string) => void
  // label names the field, and listLabel the list under it.
  label: string
  listLabel: string
  placeholder: string
  // inputRef is the field, for a dialog to give the focus to as it opens.
  inputRef: RefObject<HTMLInputElement | null>
  itemKey: (item: Item) => string
  itemText: (item: Item) => string
  renderItem: (item: Item) => ReactNode
  // onHighlight hears the item highlighted, as the highlight moves.
  onHighlight?: (item: Item | undefined) => void
  onChoose: (item: Item) => void
  // empty is what is said while nothing matches.
  empty: ReactNode
  // leading is drawn before the field, inside its row: an icon.
  leading?: ReactNode
  // onKeyDown hears a key in the field before the list does, to answer one the
  // list leaves alone; one it prevents the default of goes no further.
  onKeyDown?: (event: KeyboardEvent<HTMLInputElement>) => void
  // mono sets what is typed in the code face: a path.
  mono?: boolean
}

// Autocomplete is a field over a list it always shows: typing narrows the
// list, Up and Down move the highlight through it, the first match
// highlighted from the start, and Enter or a press chooses the one
// highlighted. The focus stays in the field throughout.
export function Autocomplete<Item>({
  groups,
  typed,
  onType,
  label,
  listLabel,
  placeholder,
  inputRef,
  itemKey,
  itemText,
  renderItem,
  onHighlight,
  onChoose,
  empty,
  leading,
  onKeyDown,
  mono = false,
}: AutocompleteProps<Item>) {
  return (
    <Base.Root
      inline
      open
      items={groups}
      filteredItems={groups}
      itemToStringValue={itemText}
      value={typed}
      onValueChange={(value, details) => {
        if (details.reason !== 'item-press') {
          onType(value)
        }
      }}
      autoHighlight="always"
      keepHighlight
      onItemHighlighted={onHighlight}
    >
      <div className="flex items-center gap-3 border-b border-border px-4">
        {leading}
        <Base.Input
          ref={inputRef}
          aria-label={label}
          placeholder={placeholder}
          spellCheck={false}
          onKeyDown={(event) => {
            onKeyDown?.(event)
            if (event.defaultPrevented) {
              event.preventBaseUIHandler()
            }
          }}
          className={cn(
            'min-w-0 flex-1 bg-transparent py-3.5 text-lg placeholder:text-muted-foreground focus-visible:outline-none',
            mono && 'font-mono',
          )}
        />
      </div>
      <Base.List
        aria-label={listLabel}
        className="max-h-[min(24rem,60dvh)] overflow-y-auto py-1.5 empty:hidden"
      >
        {(group: AutocompleteGroup<Item>) => (
          <Group
            key={group.name}
            group={group}
            itemKey={itemKey}
            renderItem={renderItem}
            onChoose={onChoose}
          />
        )}
      </Base.List>
      <Base.Empty className="text-sm text-muted-foreground empty:hidden">
        <p className="px-4 py-3">{empty}</p>
      </Base.Empty>
    </Base.Root>
  )
}

interface GroupProps<Item> {
  group: AutocompleteGroup<Item>
  itemKey: (item: Item) => string
  renderItem: (item: Item) => ReactNode
  onChoose: (item: Item) => void
}

// Group is one group of the list under its name: each item a press or Enter
// chooses, the one highlighted marked by its ground.
function Group<Item>({ group, itemKey, renderItem, onChoose }: GroupProps<Item>) {
  return (
    <Base.Group items={group.items} className="py-1">
      <Base.GroupLabel className="px-4 pt-1 pb-0.5 text-xs text-muted-foreground">
        {group.name}
      </Base.GroupLabel>
      <Base.Collection>
        {(item: Item) => (
          <Base.Item
            key={itemKey(item)}
            value={item}
            onClick={() => {
              onChoose(item)
            }}
            className={cn(
              'group relative flex cursor-pointer items-center justify-between gap-group px-4 py-1.5 text-sm',
              'hover:bg-accent/60 data-highlighted:bg-accent',
            )}
          >
            {renderItem(item)}
          </Base.Item>
        )}
      </Base.Collection>
    </Base.Group>
  )
}
