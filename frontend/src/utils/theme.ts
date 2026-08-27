export type ThemeMode = 'light' | 'dark'
export type ThemePreference = ThemeMode | 'system'

export function initialThemePreference(): ThemePreference {
  const stored = localStorage.getItem('redisshake-theme')
  if (stored === 'light' || stored === 'dark' || stored === 'system') return stored
  return 'system'
}

export function systemTheme(): ThemeMode {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function resolveTheme(preference: ThemePreference, systemMode: ThemeMode = systemTheme()): ThemeMode {
  return preference === 'system' ? systemMode : preference
}
