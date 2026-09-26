import clsx from 'clsx'
import {
  Bot,
  CircleCheck,
  CircleDot,
  CircleSlash,
  DatabaseZap,
  Factory,
  Info,
  OctagonAlert,
  Search,
  TriangleAlert,
  Wrench,
  Zap,
} from 'lucide-react'
import type { ReactNode } from 'react'
import type { AnomalyStatus, AnomalyType, MeterStatus, Severity } from '../api/types'
import { confidence as fmtConfidence } from '../lib/format'
import {
  anomalyStatusLabel,
  confidenceLevel,
  explanationSource,
  meterStatusLabel,
  severityLabel,
  typeLabel,
} from '../lib/labels'

function Pill({ className, icon, children, title }: { className: string; icon: ReactNode; children: ReactNode; title?: string }) {
  return (
    <span
      title={title}
      className={clsx('inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap', className)}
    >
      {icon}
      {children}
    </span>
  )
}

const iconCls = 'size-3.5 shrink-0'

export function SeverityBadge({ severity }: { severity: Severity }) {
  const map = {
    HIGH: { cls: 'bg-critical-soft text-ink', icon: <OctagonAlert className={clsx(iconCls, 'text-critical')} aria-hidden /> },
    MEDIUM: { cls: 'bg-serious-soft text-ink', icon: <TriangleAlert className={clsx(iconCls, 'text-serious')} aria-hidden /> },
    LOW: { cls: 'bg-neutral-soft text-ink-2', icon: <Info className={clsx(iconCls, 'text-ink-3')} aria-hidden /> },
  }[severity]
  return (
    <Pill className={map.cls} icon={map.icon}>
      {severityLabel[severity]}
    </Pill>
  )
}

export function TypeBadge({ type }: { type: AnomalyType }) {
  const icon = {
    REAL_ANOMALY: <Zap className={iconCls} aria-hidden />,
    EXPLAINABLE_ANOMALY: <Factory className={iconCls} aria-hidden />,
    DATA_QUALITY: <DatabaseZap className={iconCls} aria-hidden />,
    FALSE_POSITIVE: <CircleSlash className={iconCls} aria-hidden />,
  }[type]
  return (
    <Pill className="border border-line bg-raised text-ink-2" icon={icon}>
      {typeLabel[type]}
    </Pill>
  )
}

export function MeterStatusBadge({ status }: { status: MeterStatus }) {
  const map = {
    CRITICAL: { cls: 'bg-critical-soft text-ink', icon: <OctagonAlert className={clsx(iconCls, 'text-critical')} aria-hidden /> },
    ALERT: { cls: 'bg-serious-soft text-ink', icon: <TriangleAlert className={clsx(iconCls, 'text-serious')} aria-hidden /> },
    OK: { cls: 'bg-good-soft text-ink', icon: <CircleCheck className={clsx(iconCls, 'text-good')} aria-hidden /> },
  }[status]
  return (
    <Pill className={map.cls} icon={map.icon}>
      {meterStatusLabel[status]}
    </Pill>
  )
}

export function AnomalyStatusBadge({ status }: { status: AnomalyStatus }) {
  const icon = {
    OPEN: <CircleDot className={iconCls} aria-hidden />,
    INVESTIGATING: <Search className={iconCls} aria-hidden />,
    RESOLVED: <CircleCheck className={clsx(iconCls, 'text-good')} aria-hidden />,
    DISMISSED: <CircleSlash className={iconCls} aria-hidden />,
  }[status]
  return (
    <Pill className="bg-neutral-soft text-ink-2" icon={icon}>
      {anomalyStatusLabel[status]}
    </Pill>
  )
}

export function ConfidenceMeter({ value, compact }: { value: number; compact?: boolean }) {
  const level = confidenceLevel(value)
  return (
    <div className={clsx('flex items-center gap-2', compact ? 'min-w-24' : 'min-w-36')} title={`Confianza ${level.toLowerCase()}`}>
      <div
        className="h-1.5 flex-1 overflow-hidden rounded-full bg-accent-soft"
        role="meter"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(value * 100)}
        aria-label="Confianza de la IA"
      >
        <div className="h-full rounded-full bg-accent" style={{ width: `${value * 100}%` }} />
      </div>
      <span className="tabular text-xs font-medium text-ink">{fmtConfidence(value)}</span>
      {!compact && <span className="text-xs text-ink-3">{level}</span>}
    </div>
  )
}

export function SourceBadge({ source }: { source: string }) {
  const s = explanationSource(source)
  return (
    <Pill
      className={s.ai ? 'bg-accent-soft text-accent-ink' : 'bg-neutral-soft text-ink-2'}
      icon={s.ai ? <Bot className={iconCls} aria-hidden /> : <Wrench className={iconCls} aria-hidden />}
      title={s.ai ? 'Redactado por un modelo de lenguaje local a partir de la evidencia calculada' : 'Redactado por plantillas a partir de la evidencia calculada'}
    >
      {s.label}
    </Pill>
  )
}
