import { describe, expect, it, jest } from '@jest/globals'
import { fireEvent, render, screen } from '@testing-library/react'

import PageScaffold from '@/components/PageScaffold'

describe('PageScaffold', () => {
  it('keeps title, action, error and content in one shared page skeleton', () => {
    const retry = jest.fn()
    render(
      <PageScaffold title="连接管理" description="管理 Redis 连接。" actions={<button type="button">新建连接</button>} error="请求失败" onRetry={retry}>
        <div>连接列表</div>
      </PageScaffold>,
    )

    expect(screen.getByRole('heading', { name: '连接管理', level: 1 })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '新建连接' })).toBeInTheDocument()
    expect(screen.getByText('连接列表')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('请求失败')
    fireEvent.click(screen.getByRole('button', { name: '重试' }))
    expect(retry).toHaveBeenCalledTimes(1)
  })
})
