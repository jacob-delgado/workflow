import { useEffect, useRef } from 'react'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Input, Select } from '@/lib/Field.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { useViews } from '@/features/issues/issueApi.ts'

interface ListControlsProps {
  filter: string
  onFilter: (filter: string) => void
}

// IssueListControls sits above the issue list: the view it is of, and a filter
// over the issues already loaded, as the interface's `/` filters its list.
export function IssueListControls({ filter, onFilter }: ListControlsProps) {
  const search = useRef<HTMLInputElement>(null)
  const shortcut = useShortcut('search-issues', search, 'focus')

  return (
    <div className="flex flex-wrap items-center gap-group">
      <ViewSelect />
      <label className="flex items-center gap-item text-sm">
        <span className="text-muted-foreground">Search</span>
        <Input
          ref={search}
          size="sm"
          type="search"
          aria-keyshortcuts={shortcut}
          value={filter}
          placeholder="Key or summary"
          onChange={(event) => {
            onFilter(event.target.value)
          }}
          className="w-64"
        />
      </label>
    </div>
  )
}

// ViewSelect chooses the issue view the stream carries. It offers only the
// views the server lists, so the stream is never asked for one it refuses; a
// chosen view a configuration save has since removed falls back to the default.
// While the views cannot be read there is nothing to offer, and the list stays
// on the view it has.
function ViewSelect() {
  const { data } = useViews()
  const view = useUiStore((state) => state.view)
  const setView = useUiStore((state) => state.setView)
  const names = data?.views.map((listed) => listed.name) ?? []
  const vanished = view !== null && data !== undefined && !names.includes(view)
  const select = useRef<HTMLSelectElement>(null)
  const shortcut = useShortcut('switch-view', select, 'focus')

  useEffect(() => {
    if (vanished) {
      setView(null)
    }
  }, [vanished, setView])

  if (names.length === 0) {
    return null
  }

  return (
    <label className="flex items-center gap-item text-sm">
      <span className="text-muted-foreground">View</span>
      <Select
        ref={select}
        size="sm"
        aria-keyshortcuts={shortcut}
        value={view ?? names[0]}
        onChange={(event) => {
          setView(event.target.value)
        }}
      >
        {names.map((name) => (
          <option key={name} value={name}>
            {name}
          </option>
        ))}
      </Select>
    </label>
  )
}
