import { WarningCircle } from '@phosphor-icons/react'

export default function InlineError({ message, onRetry, className = '' }: { message: string; onRetry: () => void; className?: string }) {
  return (
    <div className={`inline-error ${className}`.trim()} role="alert">
      <WarningCircle size={21} weight="fill" />
      <div><strong>数据加载失败</strong><span>{message}</span></div>
      <button type="button" onClick={onRetry}>重试</button>
    </div>
  )
}
