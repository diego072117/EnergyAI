import type { AnomalyStatus, AnomalyType, EventType, MeterStatus, Severity } from '../api/types'

export const typeLabel: Record<AnomalyType, string> = {
  REAL_ANOMALY: 'Anomalía real',
  EXPLAINABLE_ANOMALY: 'Anomalía explicable',
  DATA_QUALITY: 'Calidad de datos',
  FALSE_POSITIVE: 'Falso positivo',
}

export const typeDescription: Record<AnomalyType, string> = {
  REAL_ANOMALY: 'Cambio real sin causa operativa conocida',
  EXPLAINABLE_ANOMALY: 'Cambio real explicado por un evento operativo',
  DATA_QUALITY: 'Lecturas del medidor no confiables',
  FALSE_POSITIVE: 'Descartado: lo explica un evento programado',
}

export const typeAction: Record<AnomalyType, string> = {
  REAL_ANOMALY: 'Investigar',
  DATA_QUALITY: 'Validar medidor',
  EXPLAINABLE_ANOMALY: 'Validar operación',
  FALSE_POSITIVE: 'No escalar',
}

export const severityLabel: Record<Severity, string> = { HIGH: 'Alta', MEDIUM: 'Media', LOW: 'Baja' }

export const meterStatusLabel: Record<MeterStatus, string> = { OK: 'Normal', ALERT: 'Alerta', CRITICAL: 'Crítico' }

export const anomalyStatusLabel: Record<AnomalyStatus, string> = {
  OPEN: 'Abierta',
  INVESTIGATING: 'En investigación',
  RESOLVED: 'Resuelta',
  DISMISSED: 'Descartada',
}

export const eventTypeLabel: Record<string, string> = {
  OPERATIONAL_CHANGE: 'Cambio operativo',
  SCHEDULED_OUTAGE: 'Parada programada',
  DATA_QUALITY: 'Calidad de datos',
  UNKNOWN: 'Sin evento reportado',
}

export function eventLabel(t: EventType): string {
  return eventTypeLabel[t] ?? t
}

export function confidenceLevel(v: number): 'Alta' | 'Media' | 'Baja' {
  if (v >= 0.85) return 'Alta'
  if (v >= 0.65) return 'Media'
  return 'Baja'
}

export function explanationSource(src: string): { ai: boolean; label: string } {
  if (src.startsWith('LLM:')) return { ai: true, label: `IA generativa · ${src.slice(4)}` }
  return { ai: false, label: 'Motor de reglas' }
}
