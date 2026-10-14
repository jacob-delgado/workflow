import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// showSection opens a section of Settings from the list beside it, once the
// list is drawn.
export async function showSection(name: string): Promise<void> {
  await userEvent.click(await screen.findByRole('tab', { name }))
}

// editSomething changes one setting, Jira's project, so a save has something
// to send: Save is held while nothing has changed.
export async function editSomething(): Promise<void> {
  await userEvent.type(await screen.findByRole('textbox', { name: 'Project' }), 'X')
}
