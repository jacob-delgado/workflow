import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// showSection opens a section of Settings from the list beside it, once the
// list is drawn.
export async function showSection(name: string): Promise<void> {
  await userEvent.click(await screen.findByRole('tab', { name }))
}
