import clsx from 'clsx'
import { ArrowRight, BrainCircuit, Check, CircleX, LoaderCircle, X } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router'
import { useAnomalies } from '../api/hooks'
import type { AnalysisStep } from '../api/types'
import { SourceBadge } from '../components/badges'
import { Button } from '../components/ui'
import { confidence } from '../lib/format'
import { typeLabel } from '../lib/labels'
import { useAnalysis } from './useAnalysis'

function stepDuration(s: AnalysisStep): string | null {
  if (!s.started_at || !s.ended_at) return null
  const ms = new Date(s.ended_at).getTime() - new Date(s.started_at).getTime()
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`
}

function StepIcon({ state }: { state: AnalysisStep['state'] }) {
  const base = 'flex size-6 shrink-0 items-center justify-center rounded-full'
  if (state === 'DONE')
    return (
      <span className={clsx(base, 'bg-accent text-white')}>
        <Check className="size-3.5" aria-hidden />
      </span>
    )
  if (state === 'RUNNING')
    return (
      <span className={clsx(base, 'bg-accent-soft text-accent')}>
        <LoaderCircle className="size-3.5 animate-spin" aria-hidden />
      </span>
    )
  if (state === 'FAILED')
    return (
      <span className={clsx(base, 'bg-critical-soft text-critical')}>
        <CircleX className="size-3.5" aria-hidden />
      </span>
    )
  return <span className={clsx(base, 'border border-line-strong bg-raised')} />
}

const stateLabel = { DONE: 'completado', RUNNING: 'en curso', FAILED: 'falló', PENDING: 'pendiente' }

export function AnalysisDialog() {
  const { run, open, setOpen, error, starting } = useAnalysis()
  const navigate = useNavigate()
  const closeRef = useRef<HTMLButtonElement>(null)
  const completed = run?.status === 'COMPLETED'
  const top = useAnomalies({}, open && completed)
  const topAnomaly = completed ? top.data?.items.find((a) => a.anomaly) : undefined

  useEffect(() => {
    if (!open) return
    closeRef.current?.focus()
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, setOpen])

  if (!open) return null

  const go = (to: string) => {
    setOpen(false)
    navigate(to)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-[2px]" onMouseDown={() => setOpen(false)}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="analysis-title"
        className="w-full max-w-lg rounded-2xl border border-line bg-surface shadow-xl"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4 border-b border-line px-6 py-4">
          <div>
            <h2 id="analysis-title" className="flex items-center gap-2 font-semibold text-ink">
              <BrainCircuit className="size-5 text-accent" aria-hidden />
              Análisis de IA
            </h2>
            <p className="mt-0.5 text-xs text-ink-3">Datos → Análisis → Anomalía → Explicación → Priorización → Acción</p>
          </div>
          <button ref={closeRef} onClick={() => setOpen(false)} className="rounded-md p-1 text-ink-3 hover:bg-hover hover:text-ink" aria-label="Cerrar">
            <X className="size-4" />
          </button>
        </div>

        <div className="px-6 py-5">
          {error && (
            <p role="alert" className="rounded-lg bg-critical-soft px-3 py-2 text-sm text-ink">
              {error}
            </p>
          )}
          {!error && !run && starting && (
            <p className="flex items-center gap-2 text-sm text-ink-2">
              <LoaderCircle className="size-4 animate-spin" aria-hidden /> Iniciando análisis…
            </p>
          )}
          {run && (
            <ol className="space-y-0" aria-live="polite">
              {run.steps.map((s, i) => (
                <li key={s.name} className="relative flex gap-3 pb-4 last:pb-0">
                  {i < run.steps.length - 1 && (
                    <span className={clsx('absolute top-6 left-3 h-[calc(100%-1.5rem)] w-px', s.state === 'DONE' ? 'bg-accent' : 'bg-line-strong')} aria-hidden />
                  )}
                  <StepIcon state={s.state} />
                  <div className="min-w-0 flex-1 pt-0.5">
                    <div className="flex items-baseline justify-between gap-2">
                      <span className={clsx('text-sm font-medium', s.state === 'PENDING' ? 'text-ink-3' : 'text-ink')}>
                        {i + 1}. {s.label}
                        <span className="sr-only"> — {stateLabel[s.state]}</span>
                      </span>
                      {stepDuration(s) && <span className="tabular text-xs text-ink-3">{stepDuration(s)}</span>}
                    </div>
                    {s.detail && <p className="mt-0.5 text-xs text-ink-2">{s.detail}</p>}
                  </div>
                </li>
              ))}
            </ol>
          )}
          {run?.status === 'FAILED' && (
            <p role="alert" className="mt-4 rounded-lg bg-critical-soft px-3 py-2 text-sm text-ink">
              El análisis falló: {run.error}
            </p>
          )}
          {completed && run.summary && (
            <div className="mt-5 rounded-xl border border-line bg-page p-4">
              <p className="text-base font-semibold text-ink">{run.summary.text}</p>
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-ink-2">
                {Object.entries(run.summary.by_type).map(([t, n]) => (
                  <span key={t}>
                    {typeLabel[t as keyof typeof typeLabel] ?? t}: <strong className="text-ink">{n}</strong>
                  </span>
                ))}
                <span>
                  Confianza media: <strong className="text-ink">{confidence(run.summary.avg_confidence)}</strong>
                </span>
              </div>
              {run.summary.explanation_source && (
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {run.summary.explanation_source.split(', ').map((s) => (
                    <SourceBadge key={s} source={s} />
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        {completed && (
          <div className="flex flex-wrap justify-end gap-2 border-t border-line px-6 py-4">
            <Button onClick={() => go('/anomalies')}>Ver todas las anomalías</Button>
            {topAnomaly && (
              <Button variant="primary" onClick={() => go(`/anomalies/${topAnomaly.id}`)}>
                Investigar {topAnomaly.meter_id} <ArrowRight className="size-4" aria-hidden />
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
