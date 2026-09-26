import { Table2, ChartLine } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import {
  Area,
  Bar,
  BarChart,
  CartesianGrid,
  ComposedChart,
  Line,
  ReferenceArea,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { ReadingPoint, ReadingSeries } from '../api/types'
import { dateTime, day, num, pct } from '../lib/format'

export type Metric = 'consumption' | 'voltage' | 'current' | 'pf'

const metricMeta: Record<Metric, { label: string; unit: string; decimals: number }> = {
  consumption: { label: 'Consumo', unit: 'kWh', decimals: 1 },
  voltage: { label: 'Voltaje', unit: 'V', decimals: 1 },
  current: { label: 'Corriente', unit: 'A', decimals: 0 },
  pf: { label: 'Factor de potencia', unit: '', decimals: 2 },
}

const axisTick = { fill: 'var(--ink-3)', fontSize: 11 }

interface Row {
  t: number
  actual: number
  expected: number
  band?: [number, number]
}

function toRows(points: ReadingPoint[], metric: Metric): Row[] {
  return points.map((p) => {
    const t = Date.parse(p.timestamp)
    switch (metric) {
      case 'voltage':
        return { t, actual: p.voltage_v, expected: p.expected_voltage_v }
      case 'current':
        return { t, actual: p.current_a, expected: p.expected_current_a }
      case 'pf':
        return { t, actual: p.power_factor, expected: p.expected_power_factor }
      default:
        return { t, actual: p.consumption_kwh, expected: p.expected_kwh, band: [p.lower_kwh, p.upper_kwh] }
    }
  })
}

function dayTicks(rows: Row[]): number[] {
  const out: number[] = []
  let last = ''
  for (const r of rows) {
    const d = new Date(r.t).toISOString().slice(0, 10)
    if (d !== last) {
      out.push(Date.parse(d))
      last = d
    }
  }
  const step = Math.ceil(out.length / 8)
  return out.filter((_, i) => i % step === 0)
}

function LegendItem({ swatch, label }: { swatch: ReactNode; label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-ink-2">
      {swatch}
      {label}
    </span>
  )
}

const lineSwatch = <span className="h-0.5 w-4 rounded-full bg-accent" aria-hidden />
const dashedSwatch = <span className="w-4 border-t-2 border-dashed border-ink-3" aria-hidden />
const bandSwatch = <span className="h-2.5 w-4 rounded-sm bg-accent/15" aria-hidden />
const windowSwatch = <span className="h-2.5 w-4 rounded-sm bg-critical/15" aria-hidden />

export interface Highlight {
  start: string
  end: string
}

export interface EventMarker {
  timestamp: string
  label: string
}

function ChartFrame({
  legend,
  table,
  children,
}: {
  legend: ReactNode
  table: ReactNode
  children: ReactNode
}) {
  const [view, setView] = useState<'chart' | 'table'>('chart')
  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1">{legend}</div>
        <button
          type="button"
          onClick={() => setView(view === 'chart' ? 'table' : 'chart')}
          className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-ink-2 hover:bg-hover hover:text-ink"
        >
          {view === 'chart' ? <Table2 className="size-3.5" aria-hidden /> : <ChartLine className="size-3.5" aria-hidden />}
          {view === 'chart' ? 'Ver tabla' : 'Ver gráfico'}
        </button>
      </div>
      {view === 'chart' ? children : <div className="max-h-72 overflow-auto rounded-lg border border-line">{table}</div>}
    </div>
  )
}

function DataTable({ head, rows }: { head: string[]; rows: (string | number)[][] }) {
  return (
    <table className="w-full text-xs">
      <thead className="sticky top-0 bg-surface text-left text-ink-3">
        <tr>
          {head.map((h) => (
            <th key={h} className="px-3 py-2 font-medium">
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody className="tabular text-ink-2">
        {rows.map((r, i) => (
          <tr key={i} className="border-t border-line">
            {r.map((c, j) => (
              <td key={j} className="px-3 py-1.5">
                {c}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function SeriesTooltip({
  active,
  payload,
  metric,
  resolution,
}: {
  active?: boolean
  payload?: { payload: Row }[]
  metric: Metric
  resolution: 'hour' | 'day'
}) {
  if (!active || !payload?.length) return null
  const r = payload[0].payload
  const m = metricMeta[metric]
  const dev = r.expected ? ((r.actual - r.expected) / r.expected) * 100 : 0
  return (
    <div className="rounded-lg border border-line bg-raised px-3 py-2 text-xs shadow-lg">
      <div className="mb-1 font-medium text-ink">{resolution === 'day' ? day(new Date(r.t)) : dateTime(new Date(r.t))}</div>
      <div className="flex items-center gap-2 text-ink-2">
        {lineSwatch} Real: <strong className="text-ink">{num(r.actual, m.decimals)} {m.unit}</strong>
      </div>
      <div className="flex items-center gap-2 text-ink-2">
        {dashedSwatch} Esperado: <span className="text-ink">{num(r.expected, m.decimals)} {m.unit}</span>
      </div>
      <div className="mt-1 text-ink-3">Desviación {pct(dev)}</div>
    </div>
  )
}

export function SeriesChart({
  series,
  metric,
  highlights = [],
  events = [],
  height = 300,
}: {
  series: ReadingSeries
  metric: Metric
  highlights?: Highlight[]
  events?: EventMarker[]
  height?: number
}) {
  const rows = useMemo(() => toRows(series.points, metric), [series.points, metric])
  const ticks = useMemo(() => dayTicks(rows), [rows])
  const m = metricMeta[metric]
  const nominal = series.thresholds.nominal_voltage
  const tol = (series.thresholds.voltage_tolerance_pct / 100) * nominal
  const unit = metric === 'consumption' ? (series.resolution === 'day' ? 'kWh/día' : 'kWh/h') : m.unit

  const legend = (
    <>
      <LegendItem swatch={lineSwatch} label={`${m.label} real`} />
      <LegendItem swatch={dashedSwatch} label="Esperado (baseline)" />
      {metric === 'consumption' && <LegendItem swatch={bandSwatch} label={`Banda normal ±${num(series.thresholds.shift_pct)}%`} />}
      {metric === 'voltage' && <LegendItem swatch={bandSwatch} label={`Rango nominal ${num(nominal)} V ±${num(series.thresholds.voltage_tolerance_pct)}%`} />}
      {highlights.length > 0 && <LegendItem swatch={windowSwatch} label="Ventana de la anomalía" />}
    </>
  )

  const table = (
    <DataTable
      head={['Fecha', `Real (${unit})`, `Esperado (${unit})`, 'Desviación']}
      rows={rows.map((r) => [
        series.resolution === 'day' ? day(new Date(r.t)) : dateTime(new Date(r.t)),
        num(r.actual, m.decimals),
        num(r.expected, m.decimals),
        pct(r.expected ? ((r.actual - r.expected) / r.expected) * 100 : 0),
      ])}
    />
  )

  return (
    <ChartFrame legend={legend} table={table}>
      <div style={{ height }} role="img" aria-label={`${m.label} real frente al esperado`}>
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={rows} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
            <CartesianGrid stroke="var(--grid)" vertical={false} />
            <XAxis
              dataKey="t"
              type="number"
              scale="time"
              domain={['dataMin', 'dataMax']}
              ticks={ticks}
              tickFormatter={(t: number) => day(new Date(t))}
              tick={axisTick}
              axisLine={{ stroke: 'var(--axis)' }}
              tickLine={false}
              minTickGap={16}
            />
            <YAxis
              tick={axisTick}
              axisLine={false}
              tickLine={false}
              width={48}
              domain={metric === 'pf' ? [(min: number) => Math.max(0, Math.floor(min * 10) / 10), 1] : ['auto', 'auto']}
              tickFormatter={(v: number) => num(v, metric === 'pf' ? 2 : 0)}
            />
            {highlights.map((h) => (
              <ReferenceArea key={h.start} x1={Date.parse(h.start)} x2={Date.parse(h.end)} fill="var(--critical)" fillOpacity={0.08} ifOverflow="hidden" />
            ))}
            {metric === 'voltage' && <ReferenceArea y1={nominal - tol} y2={nominal + tol} fill="var(--accent)" fillOpacity={0.08} />}
            {metric === 'consumption' && <Area dataKey="band" stroke="none" fill="var(--accent)" fillOpacity={0.1} isAnimationActive={false} activeDot={false} />}
            {events.map((e) => (
              <ReferenceLine
                key={e.timestamp + e.label}
                x={Date.parse(e.timestamp)}
                stroke="var(--ink-3)"
                strokeWidth={1}
                label={{ value: e.label, position: 'insideTopLeft', fill: 'var(--ink-2)', fontSize: 11 }}
              />
            ))}
            <Line dataKey="expected" stroke="var(--baseline)" strokeWidth={1.5} strokeDasharray="4 3" dot={false} activeDot={false} isAnimationActive={false} />
            <Line
              dataKey="actual"
              stroke="var(--series-1)"
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4, stroke: 'var(--surface)', strokeWidth: 2 }}
              isAnimationActive={false}
            />
            <Tooltip content={<SeriesTooltip metric={metric} resolution={series.resolution} />} cursor={{ stroke: 'var(--axis)' }} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </ChartFrame>
  )
}

interface DailyRow {
  date: string
  kwh: number
  baseline_kwh: number
}

function DailyTooltip({ active, payload }: { active?: boolean; payload?: { payload: DailyRow }[] }) {
  if (!active || !payload?.length) return null
  const r = payload[0].payload
  return (
    <div className="rounded-lg border border-line bg-raised px-3 py-2 text-xs shadow-lg">
      <div className="mb-1 font-medium text-ink">{day(r.date)}</div>
      <div className="text-ink-2">
        Consumo: <strong className="text-ink">{num(r.kwh)} kWh</strong>
      </div>
      <div className="text-ink-2">Baseline: {num(r.baseline_kwh)} kWh</div>
      <div className="mt-1 text-ink-3">Desviación {pct(((r.kwh - r.baseline_kwh) / r.baseline_kwh) * 100)}</div>
    </div>
  )
}

export function FleetChart({ data, height = 240 }: { data: DailyRow[]; height?: number }) {
  const baseline = data[0]?.baseline_kwh ?? 0
  const legend = (
    <>
      <LegendItem swatch={<span className="h-2.5 w-2.5 rounded-sm bg-accent" aria-hidden />} label="Consumo diario" />
      <LegendItem swatch={dashedSwatch} label="Baseline de la flota" />
    </>
  )
  const table = (
    <DataTable
      head={['Día', 'Consumo (kWh)', 'Baseline (kWh)', 'Desviación']}
      rows={data.map((r) => [day(r.date), num(r.kwh), num(r.baseline_kwh), pct(((r.kwh - r.baseline_kwh) / r.baseline_kwh) * 100)])}
    />
  )
  return (
    <ChartFrame legend={legend} table={table}>
      <div style={{ height }} role="img" aria-label="Consumo diario de la flota frente al baseline">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
            <CartesianGrid stroke="var(--grid)" vertical={false} />
            <XAxis dataKey="date" tickFormatter={(d: string) => day(d)} tick={axisTick} axisLine={{ stroke: 'var(--axis)' }} tickLine={false} />
            <YAxis
              tick={axisTick}
              axisLine={false}
              tickLine={false}
              width={48}
              tickFormatter={(v: number) => `${num(v / 1000, 0)}k`}
              domain={[0, (max: number) => Math.ceil((max * 1.1) / 2000) * 2000]}
            />
            <Bar dataKey="kwh" fill="var(--series-1)" radius={[4, 4, 0, 0]} maxBarSize={24} isAnimationActive={false} />
            <ReferenceLine y={baseline} stroke="var(--baseline)" strokeDasharray="4 3" strokeWidth={1.5} />
            <Tooltip content={<DailyTooltip />} cursor={{ fill: 'var(--surface-hover)' }} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </ChartFrame>
  )
}
