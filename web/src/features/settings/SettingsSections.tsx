import type { ComponentProps, ReactNode } from 'react'
import type { Control } from 'react-hook-form'
import { Tab, TabList, TabPanel, Tabs } from '@/lib/baseui/Tabs.tsx'
import { cn, contentMeasure } from '@/lib/utils.ts'
import { BranchFieldset, CommitFieldset } from './fieldsets/CommitAndBranchFieldsets.tsx'
import type { Register } from './fieldsets/Field.tsx'
import { ForgeFieldset, PullRequestFieldset } from './fieldsets/ForgeAndPullRequestFieldsets.tsx'
import { JiraFieldset } from './fieldsets/JiraFieldset.tsx'
import { MessagingFieldset } from './fieldsets/MessagingFieldset.tsx'
import { TaskwarriorFieldset } from './fieldsets/TaskwarriorFieldset.tsx'
import { KeyboardFieldset, TerminalFieldset } from './fieldsets/TerminalAndKeyboardFieldsets.tsx'
import { StoreFieldset, TimingFieldset } from './fieldsets/TimingAndStoreFieldsets.tsx'
import type { SettingsValues } from './formValues.ts'
import { LocalData } from './people/LocalData.tsx'
import { PeopleAndGroups } from './people/PeopleAndGroups.tsx'

// Fields is what every fieldset is handed: the form's register and control,
// and the Jira token as read, masked, which says where the token in effect
// comes from.
interface Fields {
  register: Register
  control: Control<SettingsValues>
  storedToken: string | null
}

// A section of the configuration, which saves with the rest of the file.
interface ConfigSection {
  id: string
  title: string
  fieldsets: (fields: Fields) => ReactNode
}

// configSections are the configuration's sections, in the order the list
// offers them, each with the fieldsets its panel holds. A new setting joins
// the fieldset of the section it belongs to.
const configSections: ConfigSection[] = [
  { id: 'jira', title: 'Jira', fieldsets: (fields) => <JiraFieldset {...fields} /> },
  { id: 'messaging', title: 'Messaging', fieldsets: (fields) => <MessagingFieldset {...fields} /> },
  {
    id: 'forge',
    title: 'Forge and pull requests',
    fieldsets: ({ register }) => (
      <>
        <ForgeFieldset register={register} />
        <PullRequestFieldset register={register} />
      </>
    ),
  },
  {
    id: 'commits',
    title: 'Commits and branches',
    fieldsets: (fields) => (
      <>
        <CommitFieldset register={fields.register} />
        <BranchFieldset {...fields} />
      </>
    ),
  },
  {
    id: 'taskwarrior',
    title: 'Taskwarrior',
    fieldsets: ({ register }) => <TaskwarriorFieldset register={register} />,
  },
  {
    id: 'terminal',
    title: 'Terminal and keys',
    fieldsets: ({ register }) => (
      <>
        <TerminalFieldset register={register} />
        <KeyboardFieldset register={register} />
      </>
    ),
  },
  {
    id: 'advanced',
    title: 'Advanced',
    fieldsets: ({ register }) => (
      <>
        <TimingFieldset register={register} />
        <StoreFieldset register={register} />
      </>
    ),
  },
]

// firstSection is the section Settings opens on.
export const firstSection = 'jira'

// The rule above the sections that save on their own, in the column the
// list stands in on a wide window.
const ownSavesRule =
  'lg:relative lg:mt-4 lg:before:absolute lg:before:inset-x-3 lg:before:-top-2 lg:before:border-t lg:before:border-border'

interface SettingsSectionsProps {
  section: string
  onSection: (section: string) => void
  fields: Fields
  // form is the configuration form's own attributes.
  form: ComponentProps<'form'>
  // bar is drawn below whichever section is shown.
  bar: ReactNode
}

// SettingsSections is Settings a section at a time, chosen from the list
// beside it: the configuration's sections, which save together, then People
// and groups and Local data, which save on their own as they change.
export function SettingsSections({ section, onSection, fields, form, bar }: SettingsSectionsProps) {
  return (
    <Tabs value={section} onValueChange={onSection}>
      <TabList label="Settings sections">
        {configSections.map(({ id, title }) => (
          <Tab key={id} value={id}>
            {title}
          </Tab>
        ))}
        <Tab value="people" className={ownSavesRule}>
          People and groups
        </Tab>
        <Tab value="local-data">Local data</Tab>
      </TabList>
      <div className={cn('flex min-w-0 flex-1 flex-col gap-section', contentMeasure)}>
        <form {...form} className="flex flex-col gap-section">
          {configSections.map(({ id, fieldsets }) => (
            <TabPanel key={id} value={id}>
              {fieldsets(fields)}
            </TabPanel>
          ))}
        </form>
        <TabPanel value="people">
          <PeopleAndGroups />
        </TabPanel>
        <TabPanel value="local-data">
          <LocalData />
        </TabPanel>
        {bar}
      </div>
    </Tabs>
  )
}
