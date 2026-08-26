import type { ReactNode } from 'react'

interface PageHeaderProps {
  title: string
  description: string
  children?: ReactNode
}

export default function PageHeader({ title, description, children }: PageHeaderProps) {
  return (
    <header className="page-header">
      <div><h1>{title}</h1><p>{description}</p></div>
      {children ? <div className="page-actions">{children}</div> : null}
    </header>
  )
}
