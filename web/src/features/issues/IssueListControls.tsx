import { useEffect } from 'react'
import { Input, Select } from '@/lib/Field.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { useViews } from './issueApi.ts'

interface ListControlsProps {
  filter: string
  onFilter: (filter: string) => void
}

// IssueListControls sits above the issue list: the view it is of, and a filter
// over the issues already loaded, as the interface's `/` filters its list.
export function IssueListControls({ filter, onFilter }: ListControlsProps) {
  return (
    <div className="flex flex-wrap items-center gap-group">
      <ViewSelect />
      <label className="flex items-center gap-item text-sm">
        <span className="text-muted-foreground">Filter</span>
        <Input
          size="sm"
          type="search"
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
        size="sm"
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
