import { ArrowLeft, ArrowRight, BrainCircuit, CalendarClock, MapPin } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { useMeter, useReadings } from '../api/hooks'
import { ConfidenceMeter, MeterStatusBadge, SeverityBadge, SourceBadge, TypeBadge } from '../components/badges'
import { SeriesChart, type Metric } from '../components/charts'
import { Card, CardHeader, EmptyState, ErrorState, Kpi, PageHeader, Segmented, Skeleton } from '../components/ui'
import { dateTime, day, energy, num, pct } from '../lib/format'
import { eventLabel, typeAction } from '../lib/labels'

const metricOptions: { value: Metric; label: string }[] = [
  { value: 'consumption', label: 'Consumo' },
  { value: 'voltage', label: 'Voltaje' },
  { value: 'current', label: 'Corriente' },
  { value: 'pf', label: 'Factor de potencia' },
]

export function MeterDetailPage() {
  const { meterId = '' } = useParams()
  const meter = useMeter(meterId)
  const [metric, setMetric] = useState<Metric>('consumption')
  const [resolution, setResolution] = useState<'hour' | 'day'>('hour')
  const readings = useReadings(meterId, { resolution })

  if (meter.error) return <ErrorState error={meter.error} onRetry={() => void meter.refetch()} />
  if (meter.isLoading || !meter.data) {
    return (
      <>
        <Skeleton className="mb-6 h-10 w-72" />
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
        <Skeleton className="mt-6 h-96" />
      </>
    )
  }

  const m = meter.data
  const s = m.stats
  const highlights = m.anomalies.filter((a) => a.anomaly || a.type === 'FALSE_POSITIVE').map((a) => ({ start: a.window_start, end: a.window_end }))
  const events = m.events.map((e) => ({ timestamp: e.timestamp, label: eventLabel(e.type) }))

  return (
    <>
      <Link to="/meters" className="mb-3 inline-flex items-center gap-1 text-sm text-ink-2 hover:text-ink">
        <ArrowLeft className="size-4" aria-hidden /> Medidores
      </Link>
      <PageHeader
        title={
          <span className="flex flex-wrap items-center gap-3">
            {m.meter_id}
            <MeterStatusBadge status={m.status} />
          </span>
        }
        subtitle={
          <span className="inline-flex items-center gap-1.5">
            {m.name} <MapPin className="ml-1 size-3.5 text-ink-3" aria-hidden /> {m.location}
          </span>
        }
      />

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Kpi label="Consumo actual (24 h)" value={`${num(m.last_24h_kwh)} kWh`} hint={s.period_end ? `hasta ${dateTime(s.period_end)}` : undefined} />
        <Kpi label="Baseline diario" value={`${num(m.baseline_daily_kwh)} kWh`} hint={m.baseline ? `${day(m.baseline.from)} – ${day(new Date(Date.parse(m.baseline.to) - 1))} (${m.baseline.days} días)` : undefined} />
        <Kpi label="Variación" value={pct(m.variation_pct)} hint="frente al baseline" tone={Math.abs(m.variation_pct) >= 25 ? 'critical' : undefined} />
        <Kpi label="Total del periodo" value={energy(m.total_kwh)} hint={`${num(s.readings)} lecturas horarias`} />
      </div>
      <div className="mt-3 grid grid-cols-3 gap-3">
        <Kpi label="Voltaje medio 24 h" value={`${num(s.avg_voltage_24h, 1)} V`} />
        <Kpi label="Corriente media 24 h" value={`${num(s.avg_current_24h)} A`} />
        <Kpi label="Factor de potencia 24 h" value={num(s.avg_power_factor_24h, 2)} />
      </div>

      <div className="mt-6 grid gap-6 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <CardHeader
            title="Histórico frente al baseline"
            subtitle="Esperado = mediana de la misma hora durante los días de referencia"
            action={<Segmented size="sm" label="Resolución" value={resolution} onChange={setResolution} options={[{ value: 'hour', label: 'Hora' }, { value: 'day', label: 'Día' }]} />}
          />
          <div className="px-5 pb-5">
            <div className="mb-3 overflow-x-auto">
              <Segmented size="sm" label="Variable" value={metric} onChange={setMetric} options={metricOptions} />
            </div>
            {readings.error ? (
              <ErrorState error={readings.error} onRetry={() => void readings.refetch()} />
            ) : !readings.data ? (
              <Skeleton className="h-80" />
            ) : (
              <SeriesChart series={readings.data} metric={metric} highlights={highlights} events={events} />
            )}
          </div>
        </Card>

        <div className="space-y-6">
          <Card>
            <CardHeader title="Análisis de IA" icon={<BrainCircuit className="size-4 text-accent" aria-hidden />} />
            {m.anomalies.length === 0 ? (
              <p className="px-5 pb-5 text-sm text-ink-2">Sin hallazgos en el último análisis: el medidor opera dentro de su comportamiento esperado.</p>
            ) : (
              <ul className="divide-y divide-line border-t border-line">
                {m.anomalies.map((a) => (
                  <li key={a.id} className="px-5 py-4">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <TypeBadge type={a.type} />
                      <SeverityBadge severity={a.severity} />
                    </div>
                    <p className="mt-2 text-sm text-ink-2">{a.reason}</p>
                    <div className="mt-3">
                      <ConfidenceMeter value={a.confidence} />
                    </div>
                    <div className="mt-3 flex items-center justify-between gap-2">
                      <SourceBadge source={a.explanation_source} />
                      <Link to={`/anomalies/${a.id}`} className="inline-flex items-center gap-1 text-sm font-medium text-accent-ink hover:underline">
                        {typeAction[a.type]} <ArrowRight className="size-4" aria-hidden />
                      </Link>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Card>
          <Card>
            <CardHeader title="Eventos operativos" icon={<CalendarClock className="size-4 text-ink-3" aria-hidden />} />
            {m.events.length === 0 ? (
              <EmptyState icon={<CalendarClock className="size-5" />} title="Sin eventos reportados" />
            ) : (
              <ul className="divide-y divide-line border-t border-line">
                {m.events.map((e) => (
                  <li key={e.id} className="px-5 py-3">
                    <div className="flex items-center justify-between gap-2 text-sm">
                      <span className="font-medium text-ink">{eventLabel(e.type)}</span>
                      <span className="tabular text-xs text-ink-3">{dateTime(e.timestamp)}</span>
                    </div>
                    <p className="mt-0.5 text-xs text-ink-2">{e.description}</p>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      </div>
    </>
  )
}
