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

interface SettingsSectionsProps {
  shown: string
  onShow: (id: string) => void
  // children are the configuration's panels, in the form that saves them.
  children: ReactNode
}

// SettingsSections shows Settings a section at a time beside the list of
// them: the configuration's sections, which save together, then People and
// groups and Local data, which save as they change.
export function SettingsSections({ shown, onShow, children }: SettingsSectionsProps) {
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
            {section.name}
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
