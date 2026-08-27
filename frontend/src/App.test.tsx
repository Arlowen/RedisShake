import { afterEach, describe, expect, it } from '@jest/globals'

import { resolvePageContext } from '@/utils/navigation'
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

describe('workspace page context', () => {
  it.each([
    ['/tasks', '同步任务'],
    ['/connections', '连接管理'],
  ])('shows %s without a redundant parent label', (path, title) => {
    expect(resolvePageContext(path)).toEqual(expect.objectContaining({ title }))
    expect(resolvePageContext(path).parent).toBeUndefined()
  })

  it.each([
    ['/system', '系统', '系统信息'],
  ])('moves %s into the shared top workspace header', (path, parent, title) => {
    expect(resolvePageContext(path)).toMatchObject({ parent, title })
  })

  it('keeps nested editor pages under their resource context', () => {
    expect(resolvePageContext('/tasks/new')).toMatchObject({ parent: '同步任务', title: '创建任务' })
    expect(resolvePageContext('/connections/new')).toMatchObject({ parent: '连接管理', title: '新建连接' })
  })
})
