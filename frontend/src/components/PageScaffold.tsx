import type { ReactNode } from 'react'

import InlineError from '@/components/InlineError'

interface PageScaffoldProps {
  error?: string
  errorClassName?: string
  onRetry: () => void
  children: ReactNode
  className?: string
}

export default function PageScaffold({ error, errorClassName, onRetry, children, className }: PageScaffoldProps) {
  return (
    <div className={`page-wrap page-scaffold${className ? ` ${className}` : ''}`}>
      {error ? <InlineError className={errorClassName} message={error} onRetry={onRetry} /> : null}
      <div className="page-scaffold-content">{children}</div>
    </div>
  )
}
