import type { ReactNode } from 'react'

import InlineError from '@/components/InlineError'
import PageHeader from '@/components/PageHeader'

interface PageScaffoldProps {
  title: string
  description: string
  actions?: ReactNode
  error?: string
  errorClassName?: string
  onRetry: () => void
  children: ReactNode
  className?: string
}

export default function PageScaffold({ title, description, actions, error, errorClassName, onRetry, children, className }: PageScaffoldProps) {
  return (
    <div className={`page-wrap page-scaffold${className ? ` ${className}` : ''}`}>
      <PageHeader title={title} description={description}>{actions}</PageHeader>
      {error ? <InlineError className={errorClassName} message={error} onRetry={onRetry} /> : null}
      <div className="page-scaffold-content">{children}</div>
    </div>
  )
}
