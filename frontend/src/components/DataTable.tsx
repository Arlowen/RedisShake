import type { ReactNode } from 'react'

interface DataTableProps {
  ariaLabel: string
  headers: ReactNode[]
  children: ReactNode
  className?: string
}

export default function DataTable({ ariaLabel, headers, children, className }: DataTableProps) {
  return (
    <section className={`data-surface data-table${className ? ` ${className}` : ''}`} aria-label={ariaLabel}>
      <div className="data-table-header">{headers.map((header, index) => <span key={index}>{header}</span>)}</div>
      <div className="data-table-body">{children}</div>
    </section>
  )
}
