import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// inSection opens the Settings section named, once the form has drawn its
// list of sections.
export async function inSection(name: string): Promise<void> {
  await userEvent.click(await screen.findByRole('tab', { name }))
}
