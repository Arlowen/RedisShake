import { describe, expect, it, jest } from '@jest/globals'
import { fireEvent, render, screen } from '@testing-library/react'

import PageScaffold from '@/components/PageScaffold'

describe('PageScaffold', () => {
  it('keeps feedback and content in the shared body without duplicating the workspace title', () => {
    const retry = jest.fn()
    render(
      <PageScaffold error="请求失败" onRetry={retry}>
        <div>连接列表</div>
      </PageScaffold>,
    )

    expect(screen.queryByRole('heading')).not.toBeInTheDocument()
    expect(screen.getByText('连接列表')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('请求失败')
    fireEvent.click(screen.getByRole('button', { name: '重试' }))
    expect(retry).toHaveBeenCalledTimes(1)
  })
})
