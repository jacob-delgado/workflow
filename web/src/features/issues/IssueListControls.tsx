import { useEffect } from 'react'
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
    <div className="flex flex-wrap items-center gap-3">
      <ViewSelect />
      <label className="flex items-center gap-2 text-sm">
        <span className="text-muted-foreground">Filter</span>
        <input
          type="search"
          value={filter}
          placeholder="Key or summary"
          onChange={(event) => {
            onFilter(event.target.value)
          }}
          className="w-48 rounded-md border border-input bg-background px-2 py-1 placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
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
    <label className="flex items-center gap-2 text-sm">
      <span className="text-muted-foreground">View</span>
      <select
        value={view ?? names[0]}
        onChange={(event) => {
          setView(event.target.value)
        }}
        className="rounded-md border border-input bg-background px-2 py-1 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        {names.map((name) => (
          <option key={name} value={name}>
            {name}
          </option>
        ))}
      </select>
    </label>
  )
}
