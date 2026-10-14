import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// inSection opens the Settings section named, once the form has drawn its
// list of sections.
export async function inSection(name: string): Promise<void> {
  await userEvent.click(await screen.findByRole('tab', { name }))
}

// anEdit changes one setting, Jira's User, so the form has something to save.
export async function anEdit(): Promise<void> {
  await userEvent.type(screen.getByLabelText('User'), 'x')
}
