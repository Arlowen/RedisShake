import { FlowArrow } from '@phosphor-icons/react'
import type { ReactNode } from 'react'

export default function EmptyState({ title, description, children }: { title: string; description: string; children?: ReactNode }) {
  return (
    <div className="empty-state">
      <span className="empty-icon"><FlowArrow size={31} /></span>
      <h3>{title}</h3><p>{description}</p>{children}
    </div>
  )
}
