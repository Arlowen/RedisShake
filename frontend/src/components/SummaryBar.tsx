import type { ReactNode } from 'react'

export default function SummaryBar({ children }: { children: ReactNode }) {
  return <div className="summary-bar">{children}</div>
}
