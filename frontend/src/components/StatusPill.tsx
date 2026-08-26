export default function StatusPill({ label, tone, pulse = false }: { label: string; tone: string; pulse?: boolean }) {
  return <span className={`status-pill tone-${tone}${pulse ? ' is-pulsing' : ''}`}><span className="status-pill-dot" />{label}</span>
}
