
export type MeterStatus = 'OK' | 'ALERT' | 'CRITICAL'
export type AnomalyType = 'REAL_ANOMALY' | 'EXPLAINABLE_ANOMALY' | 'DATA_QUALITY' | 'FALSE_POSITIVE'
export type Severity = 'HIGH' | 'MEDIUM' | 'LOW'
export type AnomalyStatus = 'OPEN' | 'INVESTIGATING' | 'RESOLVED' | 'DISMISSED'
export type RunStatus = 'PENDING' | 'RUNNING' | 'COMPLETED' | 'FAILED'
export type StepState = 'PENDING' | 'RUNNING' | 'DONE' | 'FAILED'
export type EventType = 'OPERATIONAL_CHANGE' | 'SCHEDULED_OUTAGE' | 'DATA_QUALITY' | 'UNKNOWN' | string

export interface User {
  id: number
  email: string
  name: string
}

export interface LoginResult {
  token: string
  expires_at: string
  user: User
}

export interface AnomalyRef {
  id: number
  type: AnomalyType
  severity: Severity
  confidence: number
  priority_score: number
  anomaly: boolean
  status: AnomalyStatus
}

export interface MeterSummary {
  meter_id: string
  name: string
  location: string
  status: MeterStatus
  total_kwh: number
  baseline_daily_kwh: number
  last_24h_kwh: number
  variation_pct: number
  anomaly: AnomalyRef | null
}

export interface MeterStats {
  meter_id: string
  readings: number
  period_start: string
  period_end: string
  total_kwh: number
  baseline_daily_kwh: number
  last_24h_kwh: number
  variation_pct: number
  avg_voltage_24h: number
  avg_current_24h: number
  avg_power_factor_24h: number
}

export interface BaselineInfo {
  from: string
  to: string
  days: number
  daily_kwh: number
}

export interface OperationalEvent {
  id: number
  meter_id: string
  timestamp: string
  type: EventType
  description: string
}

export interface MeterDetail extends MeterSummary {
  created_at: string
  stats: MeterStats
  baseline: BaselineInfo | null
  anomalies: Anomaly[]
  events: OperationalEvent[]
}

export interface ReadingPoint {
  timestamp: string
  consumption_kwh: number
  voltage_v: number
  current_a: number
  power_factor: number
  expected_kwh: number
  lower_kwh: number
  upper_kwh: number
  expected_voltage_v: number
  expected_current_a: number
  expected_power_factor: number
}

export interface ReadingSeries {
  meter_id: string
  resolution: 'hour' | 'day'
  points: ReadingPoint[]
  baseline: BaselineInfo | null
  thresholds: { shift_pct: number; nominal_voltage: number; voltage_tolerance_pct: number }
}

export interface VariableChange {
  name: string
  label: string
  unit: string
  before: number
  after: number
  min: number
  max: number
  change_pct: number
  change_abs: number
  significant: boolean
}

export interface Signal {
  code: string
  label: string
  supports: boolean
}

export interface EventEvidence {
  id: number
  type: EventType
  timestamp: string
  description: string
  offset_hours: number
  explains: boolean
  note: string
}

export interface Evidence {
  baseline: { from: string; to: string; days: number; daily_kwh: number }
  current: { reference_time: string; last_24h_kwh: number; variation_pct: number }
  window: { start: string; end: string; hours: number; persistent: boolean }
  direction: 'UP' | 'DOWN' | 'NONE'
  deviation_pct: number
  variables: VariableChange[]
  signals: Signal[]
  events: EventEvidence[] | null
  data_quality: {
    nominal_voltage: number
    voltage_out_of_band: number
    power_inconsistency: number
    voltage_jumps: number
    missing_hours: number
    duplicates: number
    invalid_values: number
    affected_readings: number
    readings_in_window: number
    affected_pct: number
  }
  confidence: { magnitude: number; corroboration: number; context: number; value: number }
  priority: { severity: number; type: number; magnitude: number; recency: number; total: number }
}

export interface Anomaly {
  id: number
  analysis_id: string
  meter_id: string
  detected_at: string
  type: AnomalyType
  anomaly: boolean
  severity: Severity
  confidence: number
  priority_score: number
  reason: string
  recommended_action: string
  evidence_summary: string[]
  status: AnomalyStatus
  note: string
  window_start: string
  window_end: string
  evidence: Evidence
  explanation_source: string
  updated_at: string
}

export interface Meter {
  id: number
  meter_id: string
  name: string
  location: string
  status: MeterStatus
  created_at: string
}

export interface AnomalyDetail extends Anomaly {
  meter: Meter
  rank: number
  of: number
}

export interface AnomalyList {
  analysis_id: string
  items: Anomaly[]
  total: number
}

export interface AnalysisStep {
  name: string
  label: string
  state: StepState
  detail?: string
  started_at?: string
  ended_at?: string
}

export interface AnalysisSummary {
  meters_analyzed: number
  readings_analyzed: number
  detected: number
  high_priority: number
  by_type: Record<string, number>
  avg_confidence: number
  explanation_source: string
  text: string
}

export interface AnalysisRun {
  id: string
  status: RunStatus
  current_step: string
  steps: AnalysisStep[]
  summary: AnalysisSummary | null
  error?: string
  started_at: string
  finished_at?: string
}

export interface DashboardSummary {
  meters: number
  meters_by_status: Record<MeterStatus, number>
  total_consumption_kwh: number
  period_start: string | null
  period_end: string | null
  anomalies: { detected: number; high_priority: number; active: number; by_type: Record<string, number> }
  avg_confidence: number
  last_analysis: AnalysisRun | null
  top_priority: Anomaly[]
  daily_consumption: { date: string; kwh: number; baseline_kwh: number }[]
}

export interface AIStatus {
  provider: string
  model?: string
  available: boolean
  detail?: string
}
