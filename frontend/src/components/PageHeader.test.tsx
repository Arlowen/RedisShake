import { describe, expect, it } from '@jest/globals'
import { render, screen } from '@testing-library/react'

import PageHeader from '@/components/PageHeader'

describe('PageHeader', () => {
  it('keeps one compact heading and one action region', () => {
    const { container } = render(<PageHeader title="同步任务" description="创建并运行同步任务。"><button type="button">创建任务</button></PageHeader>)
    expect(screen.getByRole('heading', { name: '同步任务', level: 1 })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '创建任务' })).toBeInTheDocument()
    expect(container.querySelector('.page-eyebrow')).not.toBeInTheDocument()
    expect(container.querySelectorAll('h1')).toHaveLength(1)
  })
})
