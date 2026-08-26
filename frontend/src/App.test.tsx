import { afterEach, describe, expect, it } from '@jest/globals'

import { initialTheme } from '@/utils/theme'

afterEach(() => localStorage.clear())

describe('theme preference', () => {
  it('uses the persisted RedisShake appearance', () => {
    localStorage.setItem('redisshake-theme', 'dark')
    expect(initialTheme()).toBe('dark')
  })

  it('follows the system when no preference exists', () => {
    Object.defineProperty(window, 'matchMedia', { configurable: true, value: () => ({ matches: true }) })
    expect(initialTheme()).toBe('dark')
  })
})
