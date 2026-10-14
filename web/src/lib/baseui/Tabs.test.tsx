import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Tab, TabList, TabPanel, Tabs } from './Tabs.tsx'

// Sections is two tabs, Jira first, each over its own panel.
function Sections() {
  const [value, setValue] = useState('jira')

  return (
    <Tabs value={value} onValueChange={setValue}>
      <TabList label="Sections">
        <Tab value="jira">Jira</Tab>
        <Tab value="forge">Forge</Tab>
      </TabList>
      <TabPanel value="jira">
        <label>
          Address <input />
        </label>
      </TabPanel>
      <TabPanel value="forge">
        <label>
          Host <input />
        </label>
      </TabPanel>
    </Tabs>
  )
}

// panelOf is the tab panel a field is in.
function panelOf(field: HTMLElement): HTMLElement {
  const panel = field.closest<HTMLElement>('[role="tabpanel"]')
  if (panel === null) {
    throw new Error('the field is in no tab panel')
  }

  return panel
}

test('a tab list is named, and each panel is named by its tab', () => {
  // Arrange
  render(<Sections />)

  // Act
  const list = screen.getByRole('tablist', { name: 'Sections' })

  // Assert
  expect(list.textContent).toBe('JiraForge')
  expect(screen.getByRole('tabpanel', { name: 'Jira' }).hidden).toBe(false)
})

test('choosing a tab shows its panel and keeps the others in place, hidden', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Sections />)

  // Act
  await user.click(screen.getByRole('tab', { name: 'Forge' }))

  // Assert
  expect(screen.getByRole('tab', { name: 'Forge' }).getAttribute('aria-selected')).toBe('true')
  expect(panelOf(screen.getByLabelText('Host')).hidden).toBe(false)
  expect(panelOf(screen.getByLabelText('Address')).hidden).toBe(true)
})

test('an arrow key moves between the tabs', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Sections />)
  screen.getByRole('tab', { name: 'Jira' }).focus()

  // Act
  await user.keyboard('{ArrowRight}')

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('tab', { name: 'Forge' }))
})

test('Tab goes from the chosen tab to the first control in its panel', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Sections />)
  screen.getByRole('tab', { name: 'Jira' }).focus()

  // Act
  await user.tab()

  // Assert
  expect(document.activeElement).toBe(screen.getByLabelText('Address'))
})
