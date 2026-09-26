import { ArrowRight, BrainCircuit, CalendarClock, Gauge, OctagonAlert, Play, Sparkles, Zap } from 'lucide-react'
import { Link } from 'react-router'
import { useAnalysis } from '../analysis/useAnalysis'
import { useDashboard } from '../api/hooks'
import type { AnomalyType } from '../api/types'
import { ConfidenceMeter, SeverityBadge, TypeBadge } from '../components/badges'
import { FleetChart } from '../components/charts'
import { Button, Card, CardHeader, EmptyState, ErrorState, Kpi, PageHeader, Skeleton } from '../components/ui'
import { confidence, energy, period, relativeTime } from '../lib/format'
import { typeAction, typeDescription } from '../lib/labels'

const typeOrder: AnomalyType[] = ['REAL_ANOMALY', 'DATA_QUALITY', 'EXPLAINABLE_ANOMALY', 'FALSE_POSITIVE']

export function DashboardPage() {
  const { data, isLoading, error, refetch } = useDashboard()
  const { start, running, starting } = useAnalysis()

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (isLoading || !data) {
    return (
      <>
        <PageHeader title="Dashboard" />
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-6">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
        <Skeleton className="mt-6 h-72" />
      </>
    )
  }

  const run = data.last_analysis
  const analyzed = data.anomalies.detected > 0 || run?.status === 'COMPLETED'
  const runLabel = run ? { COMPLETED: 'Completado', FAILED: 'Falló', RUNNING: 'En curso', PENDING: 'En curso' }[run.status] : 'Pendiente'

  return (
    <>
      <PageHeader
        title="Dashboard"
        subtitle={data.period_start && data.period_end ? `${period(data.period_start, data.period_end)} · ${data.meters} medidores monitoreados` : undefined}
      />

      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
        <Kpi
          label="Medidores"
          value={data.meters}
          icon={<Gauge className="size-4" />}
          hint={`${data.meters_by_status.OK} normales · ${data.meters_by_status.ALERT} alerta · ${data.meters_by_status.CRITICAL} crítico`}
        />
        <Kpi label="Consumo" value={energy(data.total_consumption_kwh)} icon={<Zap className="size-4" />} hint="Total del periodo" />
        <Kpi label="Anomalías IA" value={analyzed ? data.anomalies.detected : '—'} icon={<BrainCircuit className="size-4" />} hint={analyzed ? 'detectadas' : 'sin análisis'} />
        <Kpi
          label="Alta prioridad"
          value={analyzed ? data.anomalies.high_priority : '—'}
          icon={<OctagonAlert className="size-4" />}
          tone={data.anomalies.high_priority > 0 ? 'critical' : undefined}
          hint="requieren atención"
        />
        <Kpi label="Confianza IA" value={analyzed ? confidence(data.avg_confidence) : '—'} icon={<Sparkles className="size-4" />} hint="promedio de hallazgos" />
        <Kpi
          label="Último análisis"
          value={<span className="text-lg">{runLabel}</span>}
          icon={<CalendarClock className="size-4" />}
          hint={run ? relativeTime(run.finished_at ?? run.started_at) : 'Ejecuta Run AI Analysis'}
        />
      </div>

      {!analyzed ? (
        <Card className="mt-6">
          <EmptyState
            icon={<BrainCircuit className="size-5" />}
            title="Aún no hay análisis de IA"
            description="Ejecuta el análisis para calcular el baseline de cada medidor, detectar anomalías, correlacionarlas con eventos operativos y priorizar qué investigar."
            action={
              <Button variant="primary" onClick={() => void start()} loading={starting || running}>
                <Play className="size-4" aria-hidden /> Run AI Analysis
              </Button>
            }
          />
        </Card>
      ) : (
        <Card className="mt-6">
          <CardHeader
            title="Requiere atención"
            subtitle={run?.summary?.text}
            icon={<OctagonAlert className="size-4 text-critical" aria-hidden />}
            action={
              <Link to="/anomalies" className="text-sm font-medium text-accent-ink hover:underline">
                Ver todas
              </Link>
            }
          />
          {data.top_priority.length === 0 ? (
            <p className="px-5 pb-5 text-sm text-ink-2">No hay anomalías de alta prioridad abiertas.</p>
          ) : (
            <ul className="divide-y divide-line border-t border-line">
              {data.top_priority.map((a, i) => (
                <li key={a.id}>
                  <Link to={`/anomalies/${a.id}`} className="group flex flex-col gap-3 px-5 py-4 hover:bg-hover md:flex-row md:items-center">
                    <span className="tabular flex size-7 shrink-0 items-center justify-center rounded-full bg-critical-soft text-xs font-semibold text-ink">
                      {i + 1}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold text-ink">{a.meter_id}</span>
                        <TypeBadge type={a.type} />
                        <SeverityBadge severity={a.severity} />
                      </div>
                      <p className="mt-1 line-clamp-2 text-sm text-ink-2">{a.reason}</p>
                    </div>
                    <div className="flex items-center gap-4 md:w-72 md:justify-end">
                      <ConfidenceMeter value={a.confidence} compact />
                      <span className="inline-flex items-center gap-1 text-sm font-medium whitespace-nowrap text-accent-ink">
                        {typeAction[a.type]} <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" aria-hidden />
                      </span>
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>
      )}

      <div className="mt-6 grid gap-6 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <CardHeader title="Consumo diario de la flota" subtitle="Suma de los 12 medidores frente a la suma de sus baselines diarios" />
          <div className="px-5 pb-5">
            <FleetChart data={data.daily_consumption} />
          </div>
        </Card>
        <Card>
          <CardHeader title="Hallazgos por tipo" subtitle="La IA no solo detecta: también descarta" />
          <ul className="divide-y divide-line border-t border-line">
            {typeOrder.map((t) => (
              <li key={t} className="flex items-center justify-between gap-3 px-5 py-3">
                <div className="min-w-0">
                  <TypeBadge type={t} />
                  <p className="mt-1 text-xs text-ink-3">{typeDescription[t]}</p>
                </div>
                <span className="tabular text-lg font-semibold text-ink">{analyzed ? (data.anomalies.by_type[t] ?? 0) : '—'}</span>
              </li>
            ))}
          </ul>
        </Card>
      </div>
    </>
  )
}
