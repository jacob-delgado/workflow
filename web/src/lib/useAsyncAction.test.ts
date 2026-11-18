import { act, renderHook } from '@testing-library/react'
import { vi } from 'vitest'
import { held } from '@/test/fakeApi.ts'
import { refusalOf, useAsyncAction } from './useAsyncAction.ts'

// writing renders the hook over a write whose answer the test gives when it
// chooses, telling onDone what it says once done.
function writing(onDone = vi.fn()) {
  const answer = held<string>()
  const view = renderHook(() =>
    useAsyncAction(() => answer.promise, {
      fallback: 'It could not be written.',
      done: (written) => `Wrote ${written}.`,
      onDone,
    }),
  )

  return { answer, view, onDone }
}

test('a run that answers is done, with what it answered and what that means', async () => {
  // Arrange
  const { answer, view, onDone } = writing()

  // Act
  await act(async () => {
    const running = view.result.current.run()
    answer.answer('PROJ-1')
    await running
  })

  // Assert
  expect(view.result.current).toMatchObject({ state: 'done', result: 'PROJ-1' })
  expect(onDone).toHaveBeenCalledWith('Wrote PROJ-1.', 'PROJ-1')
})

test('a refused run holds why, and the refusal’s code', async () => {
  // Arrange
  const { answer, view } = writing()

  // Act
  await act(async () => {
    const running = view.result.current.run()
    answer.refuse({ title: 'Conflict', status: 409, detail: 'already there', code: 'conflict' })
    await running
  })

  // Assert
  expect(view.result.current).toMatchObject({
    state: 'error',
    error: 'already there',
    code: 'conflict',
  })
})

test('a run reset while it goes is not heard when it settles', async () => {
  // Arrange
  const { answer, view, onDone } = writing()
  let running: Promise<void> = Promise.resolve()
  act(() => {
    running = view.result.current.run()
  })

  // Act
  await act(async () => {
    view.result.current.reset()
    answer.answer('PROJ-1')
    await running
  })

  // Assert
  expect(view.result.current.state).toBe('idle')
  expect(view.result.current).not.toHaveProperty('result')
  expect(onDone).not.toHaveBeenCalled()
})

test('a reset after a run lets go of what it answered', async () => {
  // Arrange
  const { answer, view } = writing()
  await act(async () => {
    const running = view.result.current.run()
    answer.answer('PROJ-1')
    await running
  })

  // Act
  act(() => {
    view.result.current.reset()
  })

  // Assert
  expect(view.result.current.state).toBe('idle')
  expect(view.result.current).not.toHaveProperty('result')
})

test('refusalOf says the first refusal among the attempts', () => {
  // Arrange
  const attempts = [
    { state: 'done', result: 'PROJ-1' },
    { state: 'error', error: 'the link was refused', code: '' },
    { state: 'error', error: 'the preview was refused', code: '' },
  ] as const

  // Act
  const refusal = refusalOf(...attempts)

  // Assert
  expect(refusal).toBe('the link was refused')
})

test('refusalOf is empty when no attempt was refused', () => {
  // Act
  const refusal = refusalOf({ state: 'idle' }, { state: 'running' })

  // Assert
  expect(refusal).toBe('')
})
