import { fireEvent, render, screen } from '@testing-library/react'
import { useRef } from 'react'
import { useFocusedControlInView } from './focus.ts'

// Page is a scrolling root holding a list that scrolls on its own and a
// control that sits straight in the root, as the app's main holds a section.
// The root keeps 8 px clear at its head and 24 px at its foot, as main keeps
// clear of the Settings Save bar.
function Page() {
  const root = useRef<HTMLDivElement>(null)
  useFocusedControlInView(root)

  return (
    <div
      ref={root}
      aria-label="Root"
      role="region"
      tabIndex={-1}
      style={{
        overflowY: 'auto',
        paddingTop: '4px',
        scrollPaddingTop: '8px',
        scrollPaddingBottom: '24px',
      }}
    >
      <ul aria-label="List" style={{ overflowY: 'auto' }}>
        <li>
          <button type="button">In the list</button>
        </li>
      </ul>
      <div aria-label="Plain" role="group">
        <button type="button">In the root</button>
      </div>
    </div>
  )
}

// Scroller is a pane's scroll position, which jsdom does not keep.
interface Scroller {
  element: HTMLElement
  scrolled: () => number
}

// laidOut gives an element the box and the scroll jsdom does not lay out: a
// pane height px tall at the window's top whose content runs past it.
function laidOut(element: HTMLElement, height = 100): Scroller {
  let scrolled = 0
  Object.defineProperty(element, 'clientHeight', { value: height })
  Object.defineProperty(element, 'scrollHeight', { value: height * 4 })
  Object.defineProperty(element, 'scrollTop', {
    get: () => scrolled,
    set: (value: number) => {
      scrolled = value
    },
  })
  element.getBoundingClientRect = () => DOMRect.fromRect({ x: 0, y: 0, width: 300, height })

  return { element, scrolled: () => scrolled }
}

// placed puts a control's box at top in the window.
function placed(name: string, top: number, height = 40): HTMLElement {
  const control = screen.getByRole('button', { name })
  control.getBoundingClientRect = () => DOMRect.fromRect({ x: 0, y: top, width: 80, height })

  return control
}

test('a control the root cuts at its foot is scrolled wholly into view, clear of the foot', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  const control = placed('In the root', 80)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(44)
})

test('a control the root keeps its foot clear of is scrolled out from under it', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  const control = placed('In the root', 50)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(14)
})

test('a control the root cuts at its head is scrolled wholly into view, clear of the head', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  const control = placed('In the root', -15)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(-23)
})

test('a control already wholly in view leaves the root where it is', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  const control = placed('In the root', 30)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(0)
})

test('a control a pane inside the root cuts is scrolled into view in that pane alone', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }), 300)
  const list = laidOut(screen.getByRole('list', { name: 'List' }))
  const control = placed('In the list', 90)

  // Act
  control.focus()

  // Assert
  expect(list.scrolled()).toBe(30)
  expect(root.scrolled()).toBe(0)
})

test('the root taking focus itself scrolls no pane outside it', () => {
  // Arrange
  render(
    <div aria-label="Outside" role="region" style={{ overflowY: 'auto' }}>
      <Page />
    </div>,
  )
  const outside = laidOut(screen.getByRole('region', { name: 'Outside' }), 50)
  laidOut(screen.getByRole('region', { name: 'Root' }))

  // Act
  screen.getByRole('region', { name: 'Root' }).focus()

  // Assert
  expect(outside.scrolled()).toBe(0)
})

test('a control taller than the root is scrolled to show its head', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  const control = placed('In the root', 30, 150)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(22)
})

test('a control a press gives focus is left under the pointer', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  const control = placed('In the root', 80)
  fireEvent.pointerDown(control)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(0)
})

test('a control that takes focus once a press has clicked is scrolled into view', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  const control = placed('In the root', 80)
  fireEvent.pointerDown(control)
  fireEvent.click(control)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(44)
})

test('a control a key gives focus after a press that never clicked is scrolled into view', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  const control = placed('In the root', 80)
  fireEvent.pointerDown(control)
  fireEvent.keyDown(control, { key: 'Tab' })

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(44)
})

test('a control taking focus back as the window does is left where it was scrolled', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  placed('In the root', 30).focus()
  fireEvent.blur(window)
  const control = placed('In the root', 80)

  // Act
  fireEvent.focusIn(control)

  // Assert
  expect(root.scrolled()).toBe(0)
})

test('a control that takes focus after the window has taken it back is scrolled into view', () => {
  // Arrange
  render(<Page />)
  laidOut(screen.getByRole('region', { name: 'Root' }), 300)
  const list = laidOut(screen.getByRole('list', { name: 'List' }))
  const kept = placed('In the root', 30)
  kept.focus()
  fireEvent.blur(window)
  fireEvent.focusIn(kept)
  const control = placed('In the list', 90)

  // Act
  control.focus()

  // Assert
  expect(list.scrolled()).toBe(30)
})

test('a control that takes focus from another is scrolled into view', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  placed('In the list', 30).focus()
  const control = placed('In the root', 80)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(44)
})

test('a control that takes focus after the window took it back outside the root is scrolled into view', () => {
  // Arrange
  render(
    <>
      <button type="button">Outside</button>
      <Page />
    </>,
  )
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  screen.getByRole('button', { name: 'Outside' }).focus()
  fireEvent.blur(window)
  const control = placed('In the root', 80)

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(44)
})

test('a control a key gives focus after the window left is scrolled into view', () => {
  // Arrange
  render(<Page />)
  const root = laidOut(screen.getByRole('region', { name: 'Root' }))
  placed('In the list', 30).focus()
  fireEvent.blur(window)
  const control = placed('In the root', 80)
  fireEvent.keyDown(control, { key: 'Tab' })

  // Act
  control.focus()

  // Assert
  expect(root.scrolled()).toBe(44)
})
