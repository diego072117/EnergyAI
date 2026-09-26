import clsx from 'clsx'
import { ArrowLeft, BrainCircuit, CalendarClock, Check, CircleCheck, CircleSlash, CircleX, ListChecks, Search, Target, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { useAnomaly, useReadings, useUpdateAnomaly } from '../api/hooks'
import type { AnomalyDetail, AnomalyStatus } from '../api/types'
import { AnomalyStatusBadge, ConfidenceMeter, SeverityBadge, SourceBadge, TypeBadge } from '../components/badges'
import { SeriesChart, type Metric } from '../components/charts'
import { Button, Card, CardHeader, ErrorState, Segmented, Skeleton } from '../components/ui'
import { confidence, dateTime, hours, num, pct } from '../lib/format'
import { eventLabel, typeDescription } from '../lib/labels'

const DAY = 86_400_000

function chartWindow(a: AnomalyDetail) {
  const start = Date.parse(a.window_start)
  const end = Date.parse(a.window_end)
  const ref = Date.parse(a.evidence.current.reference_time)
  return {
    from: new Date(start - 2 * DAY).toISOString(),
    to: new Date(Math.min(ref, end + 2 * DAY)).toISOString(),
  }
}

function Breakdown({ label, value, hint }: { label: string; value: number; hint: string }) {
  return (
    <div>
      <div className="flex items-baseline justify-between text-xs">
        <span className="font-medium text-ink">{label}</span>
        <span className="tabular text-ink-2">{Math.round(value * 100)}%</span>
      </div>
      <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-accent-soft">
        <div className="h-full rounded-full bg-accent" style={{ width: `${value * 100}%` }} />
      </div>
      <p className="mt-1 text-xs text-ink-3">{hint}</p>
    </div>
  )
}

function Actions({ anomaly }: { anomaly: AnomalyDetail }) {
  const update = useUpdateAnomaly(anomaly.id)
  const [note, setNote] = useState(anomaly.note)
  const act = (status: AnomalyStatus) => update.mutate({ status, note })
  const pending = update.isPending ? update.variables?.status : undefined

  return (
    <div className="border-t border-line px-5 py-4">
      <label htmlFor="note" className="text-xs font-medium text-ink-2">
        Nota del operador
      </label>
      <textarea
        id="note"
        value={note}
        onChange={(e) => setNote(e.target.value)}
        rows={2}
        maxLength={1000}
        placeholder="Ej.: cuadrilla asignada, orden de trabajo #123"
        className="mt-1 w-full resize-none rounded-lg border border-line-strong bg-raised px-3 py-2 text-sm text-ink outline-none placeholder:text-ink-3 focus:border-accent"
      />
      <div className="mt-3 flex flex-wrap gap-2">
        <Button variant="primary" size="sm" onClick={() => act('INVESTIGATING')} loading={pending === 'INVESTIGATING'} disabled={anomaly.status === 'INVESTIGATING'}>
          <Search className="size-3.5" aria-hidden /> Marcar en investigación
        </Button>
        <Button size="sm" onClick={() => act('RESOLVED')} loading={pending === 'RESOLVED'} disabled={anomaly.status === 'RESOLVED'}>
          <CircleCheck className="size-3.5" aria-hidden /> Resolver
        </Button>
        <Button size="sm" variant="ghost" onClick={() => act('DISMISSED')} loading={pending === 'DISMISSED'} disabled={anomaly.status === 'DISMISSED'}>
          <CircleSlash className="size-3.5" aria-hidden /> Descartar
        </Button>
        {anomaly.status !== 'OPEN' && (
          <Button size="sm" variant="ghost" onClick={() => act('OPEN')} loading={pending === 'OPEN'}>
            Reabrir
          </Button>
        )}
      </div>
      {update.isError && (
        <p role="alert" className="mt-2 text-xs text-critical">
          {update.error.message}
        </p>
      )}
      {update.isSuccess && (
        <p role="status" className="mt-2 inline-flex items-center gap-1 text-xs text-good-ink">
          <Check className="size-3.5" aria-hidden /> Acción registrada
        </p>
      )}
    </div>
  )
}

export function InvestigationPage() {
  const id = Number(useParams().id)
  const { data: a, isLoading, error, refetch } = useAnomaly(id)
  const [metric, setMetric] = useState<Metric>('consumption')
  const win = a ? chartWindow(a) : { from: undefined, to: undefined }
  const readings = useReadings(a?.meter_id ?? '', { ...win, resolution: 'hour' })

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (isLoading || !a) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-80" />
        <Skeleton className="h-40" />
        <Skeleton className="h-80" />
      </div>
    )
  }

  const ev = a.evidence
  const events = ev.events ?? []
  const supporting = ev.signals.filter((s) => s.supports).length

  return (
    <>
      <Link to="/anomalies" className="mb-3 inline-flex items-center gap-1 text-sm text-ink-2 hover:text-ink">
        <ArrowLeft className="size-4" aria-hidden /> Anomalías IA
      </Link>

      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2 text-xs text-ink-3">
            <span className="rounded-full bg-neutral-soft px-2 py-0.5 font-medium text-ink">
              Prioridad #{a.rank} de {a.of}
            </span>
            <span>Detectada desde {dateTime(a.window_start)}</span>
          </div>
          <h1 className="mt-2 text-xl font-semibold tracking-tight text-ink sm:text-2xl">
            <Link to={`/meters/${a.meter_id}`} className="hover:underline">
              {a.meter_id}
            </Link>{' '}
            <span className="font-normal text-ink-2">· {a.meter.name}</span>
          </h1>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <TypeBadge type={a.type} />
            <SeverityBadge severity={a.severity} />
            <AnomalyStatusBadge status={a.status} />
          </div>
        </div>
        <div className="rounded-xl border border-line bg-surface px-4 py-3">
          <div className="text-xs font-medium text-ink-2">Confianza de la IA</div>
          <div className="mt-1">
            <ConfidenceMeter value={a.confidence} />
          </div>
        </div>
      </div>

      <div className="grid gap-6 xl:grid-cols-3">
        <div className="space-y-6 xl:col-span-2">
          <Card>
            <CardHeader title="Qué encontró la IA" icon={<BrainCircuit className="size-4 text-accent" aria-hidden />} action={<SourceBadge source={a.explanation_source} />} />
            <div className="px-5 pb-5">
              <p className="text-[15px] leading-relaxed text-ink">{a.reason}</p>
              <p className="mt-2 text-xs text-ink-3">{typeDescription[a.type]}.</p>
            </div>
          </Card>

          <Card>
            <CardHeader
              title="Comparación contra baseline"
              subtitle={`Ventana de ${hours(ev.window.hours)}${ev.window.persistent ? ', sigue activa' : ''} · desviación media ${pct(ev.deviation_pct)}`}
            />
            <div className="px-5 pb-5">
              <div className="mb-3 overflow-x-auto">
                <Segmented
                  size="sm"
                  label="Variable"
                  value={metric}
                  onChange={setMetric}
                  options={[
                    { value: 'consumption', label: 'Consumo' },
                    { value: 'current', label: 'Corriente' },
                    { value: 'voltage', label: 'Voltaje' },
                    { value: 'pf', label: 'Factor de potencia' },
                  ]}
                />
              </div>
              {readings.data ? (
                <SeriesChart
                  series={readings.data}
                  metric={metric}
                  height={280}
                  highlights={[{ start: a.window_start, end: a.window_end }]}
                  events={events.map((e) => ({ timestamp: e.timestamp, label: eventLabel(e.type) }))}
                />
              ) : (
                <Skeleton className="h-72" />
              )}
            </div>
          </Card>

          <Card>
            <CardHeader title="Variables que cambiaron" subtitle="Promedio en la ventana frente a lo esperado por el baseline para las mismas horas" />
            <div className="overflow-x-auto">
              <table className="w-full min-w-[560px] text-sm">
                <thead className="border-y border-line text-left text-xs text-ink-3">
                  <tr>
                    <th className="px-5 py-2 font-medium">Variable</th>
                    <th className="px-4 py-2 text-right font-medium">Esperado</th>
                    <th className="px-4 py-2 text-right font-medium">Observado</th>
                    <th className="px-4 py-2 text-right font-medium">Rango</th>
                    <th className="px-5 py-2 text-right font-medium">Cambio</th>
                  </tr>
                </thead>
                <tbody className="tabular divide-y divide-line">
                  {ev.variables.map((v) => {
                    const d = v.name === 'power_factor' ? 2 : v.name === 'current_a' ? 0 : 1
                    return (
                      <tr key={v.name}>
                        <td className="px-5 py-2.5">
                          <span className={clsx('inline-flex items-center gap-1.5', v.significant ? 'font-medium text-ink' : 'text-ink-2')}>
                            {v.significant && <TriangleAlert className="size-3.5 text-serious" aria-label="Cambio relevante" />}
                            {v.label} {v.unit && <span className="text-xs text-ink-3">({v.unit})</span>}
                          </span>
                        </td>
                        <td className="px-4 py-2.5 text-right text-ink-2">{num(v.before, d)}</td>
                        <td className="px-4 py-2.5 text-right text-ink">{num(v.after, d)}</td>
                        <td className="px-4 py-2.5 text-right text-ink-3">
                          {num(v.min, d)} – {num(v.max, d)}
                        </td>
                        <td className={clsx('px-5 py-2.5 text-right', v.significant ? 'font-semibold text-ink' : 'text-ink-2')}>
                          {v.name === 'power_factor' ? `${v.change_abs > 0 ? '+' : ''}${num(v.change_abs, 2)}` : pct(v.change_pct)}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </Card>
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader title="Acción recomendada" icon={<Target className="size-4 text-accent" aria-hidden />} />
            <p className="px-5 pb-4 text-sm leading-relaxed text-ink">{a.recommended_action}</p>
            <Actions key={a.id + a.status} anomaly={a} />
          </Card>

          <Card>
            <CardHeader title="Evidencia" subtitle={`${supporting} de ${ev.signals.length} señales respaldan la clasificación`} icon={<ListChecks className="size-4 text-ink-3" aria-hidden />} />
            <ul className="space-y-2.5 px-5 pb-5">
              {ev.signals.map((s) => (
                <li key={s.code} className="flex gap-2 text-sm">
                  {s.supports ? <CircleCheck className="mt-0.5 size-4 shrink-0 text-good" aria-label="Respalda" /> : <CircleX className="mt-0.5 size-4 shrink-0 text-ink-3" aria-label="No respalda" />}
                  <span className={s.supports ? 'text-ink' : 'text-ink-3'}>{s.label}</span>
                </li>
              ))}
            </ul>
          </Card>

          <Card>
            <CardHeader title="Eventos relacionados" icon={<CalendarClock className="size-4 text-ink-3" aria-hidden />} />
            {events.length === 0 ? (
              <p className="px-5 pb-5 text-sm text-ink-2">No hay eventos operativos cerca del cambio: nada lo explica.</p>
            ) : (
              <ul className="divide-y divide-line border-t border-line">
                {events.map((e) => (
                  <li key={e.id} className="px-5 py-3">
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-sm font-medium text-ink">{eventLabel(e.type)}</span>
                      <span className="tabular text-xs text-ink-3">{dateTime(e.timestamp)}</span>
                    </div>
                    <p className="mt-0.5 text-xs text-ink-2">«{e.description}»</p>
                    <p className={clsx('mt-1.5 inline-flex items-center gap-1 text-xs font-medium', e.explains ? 'text-good-ink' : 'text-ink-2')}>
                      {e.explains ? <CircleCheck className="size-3.5" aria-hidden /> : <CircleX className="size-3.5" aria-hidden />}
                      {e.note}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <Card>
            <CardHeader title="¿Por qué esta confianza y prioridad?" subtitle={`Confianza ${confidence(ev.confidence.value)} · prioridad ${Math.round(ev.priority.total)}/100`} />
            <div className="space-y-4 px-5 pb-5">
              <Breakdown label="Magnitud" value={ev.confidence.magnitude} hint="Qué tan lejos del comportamiento normal está la lectura" />
              <Breakdown label="Corroboración" value={ev.confidence.corroboration} hint="Proporción de señales independientes que coinciden" />
              <Breakdown label="Contexto" value={ev.confidence.context} hint="Qué tan claro confirman (o descartan) los eventos una explicación" />
              <p className="border-t border-line pt-3 text-xs text-ink-3">
                Prioridad = severidad {ev.priority.severity} + tipo {ev.priority.type} + magnitud {num(ev.priority.magnitude, 1)} + recencia {ev.priority.recency}
              </p>
            </div>
          </Card>
        </div>
      </div>
    </>
  )
}
