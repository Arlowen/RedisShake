import { CheckCircle, Warning, XCircle } from '@phosphor-icons/react'

import type { CheckItem } from '@/api/types'
import StatusPill from '@/components/StatusPill'
import { checkStateMeta } from '@/utils/presentation'

const icons = { PASS: CheckCircle, WARNING: Warning, FAIL: XCircle }

export default function CheckResultPanel({ checks, title }: { checks: CheckItem[]; title?: string }) {
  return (
    <section className="check-panel">
      {title ? <header><h3>{title}</h3><span>{checks.length} 项</span></header> : null}
      <div className="check-list">
        {checks.map((item) => {
          const Icon = icons[item.state]
          return <div key={item.code} className="check-item"><Icon size={20} weight={item.state === 'PASS' ? 'fill' : 'regular'} /><span>{item.message}</span><StatusPill label={checkStateMeta[item.state].label} tone={checkStateMeta[item.state].tone} /></div>
        })}
      </div>
    </section>
  )
}
