import { z } from 'zod'
import './zodConfig.ts'

test('the schemas parse without compiling a script from a string', () => {
  // Act
  const config = z.config()

  // Assert
  expect(config.jitless).toBe(true)
})
