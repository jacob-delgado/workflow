import type { ReactNode } from 'react'
import { Tab, TabList, TabPanel, Tabs } from '@/lib/baseui/Tabs.tsx'
import { LocalData } from './people/LocalData.tsx'
import { PeopleAndGroups } from './people/PeopleAndGroups.tsx'

// A section of the configuration the Settings form edits: its name in the
// list, and the configuration's top-level keys its fields are under.
interface ConfigSection {
  id: string
  name: string
  keys: string[]
}

// configSections are the configuration's sections, in the order the list
// shows them.
const configSections: ConfigSection[] = [
  { id: 'jira', name: 'Jira', keys: ['jira'] },
  { id: 'messaging', name: 'Messaging', keys: ['messaging'] },
  { id: 'forge', name: 'Forge and pull requests', keys: ['forge', 'pull_request'] },
  { id: 'commits', name: 'Commits and branches', keys: ['commit', 'branch'] },
  { id: 'taskwarrior', name: 'Taskwarrior', keys: ['taskwarrior'] },
  { id: 'terminal', name: 'Terminal and keys', keys: ['ui'] },
  { id: 'advanced', name: 'Advanced', keys: ['timing', 'store'] },
]

// sectionOfKey is the section a setting's dotted key is in, or undefined for
// one no section shows.
export function sectionOfKey(key: string): ConfigSection | undefined {
  const top = key.split('.')[0] ?? ''

  return configSections.find((section) => section.keys.includes(top))
}

// bySection is how many of the settings named by their dotted keys each
// section holds, by its id.
export function bySection(keys: string[]): Record<string, number> {
  return Object.fromEntries(
    configSections.map((section) => [
      section.id,
      keys.filter((key) => sectionOfKey(key) === section).length,
    ]),
  )
}

interface SettingsSectionsProps {
  shown: string
  onShow: (id: string) => void
  // unsaved is how many unsaved changes each section holds, and refused how
  // many settings in it are to fix, by its id.
  unsaved: Record<string, number>
  refused: Record<string, number>
  // children are the configuration's panels, in the form that saves them.
  children: ReactNode
}

// SettingsSections shows Settings a section at a time beside the list of
// them: the configuration's sections, which save together, then People and
// groups and Local data, which save as they change.
export function SettingsSections({
  shown,
  onShow,
  unsaved,
  refused,
  children,
}: SettingsSectionsProps) {
  return (
    <Tabs
      value={shown}
      onValueChange={onShow}
      className="grid items-start gap-block lg:grid-cols-[13rem_minmax(0,42rem)]"
    >
      <TabList
        label="Settings sections"
        className="flex flex-wrap gap-1 lg:sticky lg:top-0 lg:flex-col"
      >
        {configSections.map((section) => (
          <Tab key={section.id} value={section.id}>
            <SectionName
              name={section.name}
              unsaved={unsaved[section.id] ?? 0}
              refused={refused[section.id] ?? 0}
            />
          </Tab>
        ))}
        <span aria-hidden className="mx-3 hidden border-t border-border lg:my-1 lg:block" />
        <Tab value="people">People and groups</Tab>
        <Tab value="local">Local data</Tab>
      </TabList>
      <div className="min-w-0">
        {children}
        <TabPanel value="people">
          <PeopleAndGroups />
        </TabPanel>
        <TabPanel value="local">
          <LocalData />
        </TabPanel>
      </div>
    </Tabs>
  )
}

// ConfigPanel is one section of the configuration, as the list shows it.
export function ConfigPanel({ id, children }: { id: string; children: ReactNode }) {
  return (
    <TabPanel value={id} className="flex flex-col gap-section">
      {children}
    </TabPanel>
  )
}

interface SectionNameProps {
  name: string
  unsaved: number
  refused: number
}

// SectionName is a section as the list names it: how many settings in it are
// to fix, in the failure color, or else how many are unsaved, each said to a
// screen reader after the name.
function SectionName({ name, unsaved, refused }: SectionNameProps) {
  const words = [
    refused === 0 ? '' : `${String(refused)} to fix`,
    unsaved === 0 ? '' : `${String(unsaved)} unsaved`,
  ].filter((word) => word !== '')

  return (
    <>
      <span>
        {name}
        {words.length === 0 ? null : <span className="sr-only">, {words.join(', ')}</span>}
      </span>
      {refused === 0 ? null : (
        <span aria-hidden className="shrink-0 text-xs text-destructive tabular-nums">
          {refused} to fix
        </span>
      )}
      {refused !== 0 || unsaved === 0 ? null : (
        <span aria-hidden className="text-xs text-primary tabular-nums">
          {unsaved}
        </span>
      )}
    </>
  )
}
