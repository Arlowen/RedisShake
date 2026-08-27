import { afterEach, describe, expect, it } from '@jest/globals'

import { initialThemePreference, resolveTheme } from '@/utils/theme'

afterEach(() => localStorage.clear())

describe('theme preference', () => {
  it('uses the persisted RedisShake appearance', () => {
    localStorage.setItem('redisshake-theme', 'dark')
    expect(initialThemePreference()).toBe('dark')
  })

  it('defaults to following the system', () => {
    expect(initialThemePreference()).toBe('system')
  })

  it('persists following the system as an explicit preference', () => {
    localStorage.setItem('redisshake-theme', 'system')
    expect(initialThemePreference()).toBe('system')
  })

  it('resolves the system preference using the current OS appearance', () => {
    expect(resolveTheme('system', 'dark')).toBe('dark')
    expect(resolveTheme('system', 'light')).toBe('light')
  })
})
