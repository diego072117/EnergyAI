import type { AnalysisRun, Anomaly, AnomalyDetail, DashboardSummary, MeterSummary } from '../api/types'

const evidence: Anomaly['evidence'] = {
  baseline: { from: '2026-09-01T00:00:00Z', to: '2026-09-08T00:00:00Z', days: 7, daily_kwh: 1051 },
  current: { reference_time: '2026-09-14T23:00:00Z', last_24h_kwh: 2208, variation_pct: 110.7 },
  window: { start: '2026-09-12T14:00:00Z', end: '2026-09-14T23:00:00Z', hours: 58, persistent: true },
  direction: 'UP',
  deviation_pct: 110.4,
  variables: [
    { name: 'consumption_kwh', label: 'Consumo', unit: 'kWh/h', before: 44.1, after: 92.8, min: 62.2, max: 117.5, change_pct: 110.5, change_abs: 48.7, significant: true },
    { name: 'power_factor', label: 'Factor de potencia', unit: '', before: 0.94, after: 0.74, min: 0.71, max: 0.78, change_pct: -21, change_abs: -0.2, significant: true },
  ],
  signals: [
    { code: 'LEVEL_SHIFT', label: 'Consumo +110,4% frente al baseline', supports: true },
    { code: 'NO_DATA_QUALITY', label: 'Sin problemas de calidad de datos en la ventana', supports: true },
  ],
  events: [
    { id: 3, type: 'UNKNOWN', timestamp: '2026-09-12T14:00:00Z', description: 'No operational event reported', offset_hours: 0, explains: false, note: 'Registro sin evento operativo reportado: no explica el cambio.' },
  ],
  data_quality: { nominal_voltage: 220, voltage_out_of_band: 0, power_inconsistency: 0, voltage_jumps: 0, missing_hours: 0, duplicates: 0, invalid_values: 0, affected_readings: 0, readings_in_window: 0, affected_pct: 0 },
  confidence: { magnitude: 1, corroboration: 1, context: 1, value: 0.98 },
  priority: { severity: 60, type: 25, magnitude: 7.4, recency: 5, total: 97.4 },
}

export const anomaly: Anomaly = {
  id: 1,
  analysis_id: 'run-1',
  meter_id: 'M-109',
  detected_at: '2026-09-12T14:00:00Z',
  type: 'REAL_ANOMALY',
  anomaly: true,
  severity: 'HIGH',
  confidence: 0.98,
  priority_score: 97.4,
  reason: 'Consumo +110,4% frente al baseline sin evento operativo.',
  recommended_action: 'Priorizar una inspección en sitio del medidor.',
  evidence_summary: ['Consumo +110,4%'],
  status: 'OPEN',
  note: '',
  window_start: '2026-09-12T14:00:00Z',
  window_end: '2026-09-14T23:00:00Z',
  evidence,
  explanation_source: 'LLM:qwen2.5:7b',
  updated_at: '2026-09-26T10:00:00Z',
}

export const falsePositive: Anomaly = {
  ...anomaly,
  id: 4,
  meter_id: 'M-106',
  type: 'FALSE_POSITIVE',
  anomaly: false,
  severity: 'LOW',
  confidence: 0.95,
  priority_score: 15.3,
  explanation_source: 'TEMPLATE',
}

export const anomalyDetail: AnomalyDetail = {
  ...anomaly,
  meter: { id: 9, meter_id: 'M-109', name: 'Línea de producción 3', location: 'Planta Norte', status: 'CRITICAL', created_at: '2026-09-26T00:00:00Z' },
  rank: 1,
  of: 4,
}

export const completedRun: AnalysisRun = {
  id: 'run-1',
  status: 'COMPLETED',
  current_step: '',
  steps: ['Lecturas', 'Baseline', 'Detección', 'Correlación', 'Eventos', 'Explicación', 'Recomendación'].map((label, i) => ({
    name: `S${i}`,
    label,
    state: 'DONE' as const,
    detail: `detalle ${label}`,
  })),
  summary: {
    meters_analyzed: 12,
    readings_analyzed: 4032,
    detected: 4,
    high_priority: 2,
    by_type: { REAL_ANOMALY: 1, DATA_QUALITY: 1, EXPLAINABLE_ANOMALY: 1, FALSE_POSITIVE: 1 },
    avg_confidence: 0.93,
    explanation_source: 'LLM:qwen2.5:7b',
    text: '4 anomalías detectadas · 2 requieren atención prioritaria',
  },
  started_at: '2026-09-26T10:00:00Z',
  finished_at: '2026-09-26T10:00:05Z',
}

export const dashboard: DashboardSummary = {
  meters: 12,
  meters_by_status: { OK: 9, ALERT: 2, CRITICAL: 1 },
  total_consumption_kwh: 155250,
  period_start: '2026-09-01T00:00:00Z',
  period_end: '2026-09-14T23:00:00Z',
  anomalies: { detected: 4, high_priority: 2, active: 3, by_type: { REAL_ANOMALY: 1, DATA_QUALITY: 1, EXPLAINABLE_ANOMALY: 1, FALSE_POSITIVE: 1 } },
  avg_confidence: 0.93,
  last_analysis: completedRun,
  top_priority: [anomaly],
  daily_consumption: [{ date: '2026-09-01', kwh: 10700, baseline_kwh: 10950 }],
}

export const emptyDashboard: DashboardSummary = {
  ...dashboard,
  meters_by_status: { OK: 12, ALERT: 0, CRITICAL: 0 },
  anomalies: { detected: 0, high_priority: 0, active: 0, by_type: {} },
  avg_confidence: 0,
  last_analysis: null,
  top_priority: [],
}

export const meters: MeterSummary[] = [
  {
    meter_id: 'M-109', name: 'Línea de producción 3', location: 'Planta Norte', status: 'CRITICAL', total_kwh: 17526,
    baseline_daily_kwh: 1051, last_24h_kwh: 2208, variation_pct: 110.7,
    anomaly: { id: 1, type: 'REAL_ANOMALY', severity: 'HIGH', confidence: 0.98, priority_score: 97.4, anomaly: true, status: 'OPEN' },
  },
  {
    meter_id: 'M-101', name: 'Oficinas', location: 'Sede central', status: 'OK', total_kwh: 10226,
    baseline_daily_kwh: 731, last_24h_kwh: 729, variation_pct: -0.3, anomaly: null,
  },
]
