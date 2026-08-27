import { describe, expect, it } from '@jest/globals'
import { render, screen } from '@testing-library/react'

import DataTable from '@/components/DataTable'

describe('DataTable', () => {
  it('provides the same header and data surface structure for each resource page', () => {
    render(<DataTable ariaLabel="系统运行配置" headers={['配置项', '状态', '配置值']}><div className="data-row">元数据存储</div></DataTable>)
    expect(screen.getByRole('region', { name: '系统运行配置' })).toBeInTheDocument()
    expect(screen.getByText('配置项')).toBeInTheDocument()
    expect(screen.getByText('元数据存储')).toBeInTheDocument()
  })
})
