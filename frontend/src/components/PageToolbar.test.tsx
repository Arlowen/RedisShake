import { describe, expect, it, jest } from '@jest/globals'
import { fireEvent, render, screen } from '@testing-library/react'

import PageToolbar from '@/components/PageToolbar'

describe('PageToolbar', () => {
  it('shares search, filters and refresh behavior across management pages', () => {
    const onChange = jest.fn()
    const onRefresh = jest.fn()
    render(
      <PageToolbar ariaLabel="连接管理工具栏" search={{ value: '', placeholder: '搜索连接', ariaLabel: '搜索连接', onChange }} refreshLabel="刷新连接" onRefresh={onRefresh}>
        <button type="button">全部拓扑</button>
      </PageToolbar>,
    )

    fireEvent.change(screen.getByRole('textbox', { name: '搜索连接' }), { target: { value: 'local' } })
    fireEvent.click(screen.getByRole('button', { name: '刷新连接' }))
    expect(onChange).toHaveBeenCalledWith('local')
    expect(onRefresh).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: '全部拓扑' })).toBeInTheDocument()
  })

  it('places a primary action after the refresh control', () => {
    const { container } = render(
      <PageToolbar ariaLabel="同步任务工具栏" refreshLabel="刷新任务" onRefresh={() => undefined} afterRefresh={<button type="button">创建任务</button>} />,
    )

    const controls = Array.from(container.querySelector('.toolbar-right')?.querySelectorAll('button') ?? [])
    expect(controls.map((control) => control.getAttribute('aria-label') || control.textContent)).toEqual(['刷新任务', '创建任务'])
  })
})
