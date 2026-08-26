import type { ReactNode } from 'react'

interface PageHeaderProps {
  eyebrow: string
  title: string
  description: string
  children?: ReactNode
}

export default function PageHeader({ eyebrow, title, description, children }: PageHeaderProps) {
  return (
    <header className="page-header">
      <div><span className="page-eyebrow">{eyebrow}</span><h1>{title}</h1><p>{description}</p></div>
      {children ? <div className="page-actions">{children}</div> : null}
    </header>
  )
}
