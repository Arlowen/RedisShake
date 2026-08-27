import { Button, Input } from 'antd'
import { ArrowsClockwise, MagnifyingGlass } from '@phosphor-icons/react'
import type { ReactNode } from 'react'

interface SearchControl {
  value: string
  placeholder: string
  ariaLabel: string
  onChange: (value: string) => void
}

interface PageToolbarProps {
  ariaLabel: string
  search?: SearchControl
  leading?: ReactNode
  children?: ReactNode
  afterRefresh?: ReactNode
  refreshing?: boolean
  refreshLabel: string
  onRefresh: () => void
}

export default function PageToolbar({ ariaLabel, search, leading, children, afterRefresh, refreshing, refreshLabel, onRefresh }: PageToolbarProps) {
  return (
    <section className="page-toolbar" aria-label={ariaLabel}>
      <div className="toolbar-leading">
        {search ? <Input className="page-search" allowClear prefix={<MagnifyingGlass size={16} />} value={search.value} placeholder={search.placeholder} aria-label={search.ariaLabel} onChange={(event) => search.onChange(event.target.value)} /> : leading}
      </div>
      <div className="toolbar-right">
        {children}
        <Button type="text" aria-label={refreshLabel} title={refreshLabel} loading={refreshing} icon={<ArrowsClockwise size={16} />} onClick={onRefresh} />
        {afterRefresh}
      </div>
    </section>
  )
}
