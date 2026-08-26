export type ThemeMode = 'light' | 'dark'

export function initialTheme(): ThemeMode {
  const stored = localStorage.getItem('redisshake-theme')
  if (stored === 'light' || stored === 'dark') return stored
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}
