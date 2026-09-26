import clsx from 'clsx'
import { ArrowRight, BrainCircuit, Play } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router'
import { useAnalysis } from '../analysis/useAnalysis'
import { useAnomalies } from '../api/hooks'
import { AnomalyStatusBadge, ConfidenceMeter, SeverityBadge, TypeBadge } from '../components/badges'
import { Button, Card, EmptyState, ErrorState, PageHeader, Segmented, Skeleton } from '../components/ui'
import { dateTime } from '../lib/format'
import { typeAction } from '../lib/labels'

const typeOptions = [
  { value: '', label: 'Todos' },
  { value: 'REAL_ANOMALY', label: 'Reales' },
  { value: 'DATA_QUALITY', label: 'Calidad de datos' },
  { value: 'EXPLAINABLE_ANOMALY', label: 'Explicables' },
  { value: 'FALSE_POSITIVE', label: 'Falsos positivos' },
]

const severityOptions = [
  { value: '', label: 'Todas' },
  { value: 'HIGH', label: 'Alta' },
  { value: 'MEDIUM', label: 'Media' },
  { value: 'LOW', label: 'Baja' },
]

export function AnomaliesPage() {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const type = params.get('type') ?? ''
  const severity = params.get('severity') ?? ''
  const { data, isLoading, error, refetch, isPlaceholderData } = useAnomalies({ type, severity })
  const { start, running, starting, run } = useAnalysis()

  const set = (k: string, v: string) => {
    const p = new URLSearchParams(params)
    if (v) p.set(k, v)
    else p.delete(k)
    setParams(p, { replace: true })
  }

  const noAnalysis = data && !data.analysis_id

  return (
    <>
      <PageHeader
        title="Anomalías IA"
        subtitle="Ordenadas por prioridad: qué investigar primero y por qué"
        actions={
          run?.summary && <span className="hidden rounded-full bg-accent-soft px-3 py-1 text-xs font-medium text-accent-ink sm:inline">{run.summary.text}</span>
        }
      />

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <Segmented label="Tipo" value={type} onChange={(v) => set('type', v)} options={typeOptions} />
        <Segmented label="Severidad" value={severity} onChange={(v) => set('severity', v)} options={severityOptions} />
      </div>

      <Card className="overflow-hidden">
        {error ? (
          <ErrorState error={error} onRetry={() => void refetch()} />
        ) : isLoading || !data ? (
          <div className="space-y-2 p-4">
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} className="h-14" />
            ))}
          </div>
        ) : noAnalysis ? (
          <EmptyState
            icon={<BrainCircuit className="size-5" />}
            title="Todavía no hay análisis"
            description="Ejecuta el análisis de IA para detectar y priorizar anomalías."
            action={
              <Button variant="primary" onClick={() => void start()} loading={starting || running}>
                <Play className="size-4" aria-hidden /> Run AI Analysis
              </Button>
            }
          />
        ) : data.items.length === 0 ? (
          <EmptyState icon={<BrainCircuit className="size-5" />} title="Sin anomalías para este filtro" />
        ) : (
          <div className={clsx('overflow-x-auto transition-opacity', isPlaceholderData && 'opacity-60')}>
            <table className="w-full min-w-[860px] text-sm">
              <thead className="border-b border-line text-left text-xs text-ink-3">
                <tr>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Prioridad
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Medidor
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Tipo
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Severidad
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Confianza
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Estado
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Acción
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {data.items.map((a, i) => (
                  <tr
                    key={a.id}
                    tabIndex={0}
                    onClick={() => navigate(`/anomalies/${a.id}`)}
                    onKeyDown={(e) => e.key === 'Enter' && navigate(`/anomalies/${a.id}`)}
                    className="cursor-pointer hover:bg-hover focus-visible:bg-hover"
                  >
                    <td className="px-4 py-3">
                      <span className="tabular inline-flex items-center gap-2">
                        <span className="flex size-6 items-center justify-center rounded-full bg-neutral-soft text-xs font-semibold text-ink">{i + 1}</span>
                        <span className="text-xs text-ink-3">{Math.round(a.priority_score)}/100</span>
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="font-semibold text-ink">{a.meter_id}</div>
                      <div className="text-xs text-ink-3">desde {dateTime(a.window_start)}</div>
                    </td>
                    <td className="px-4 py-3">
                      <TypeBadge type={a.type} />
                    </td>
                    <td className="px-4 py-3">
                      <SeverityBadge severity={a.severity} />
                    </td>
                    <td className="px-4 py-3">
                      <ConfidenceMeter value={a.confidence} />
                    </td>
                    <td className="px-4 py-3">
                      <AnomalyStatusBadge status={a.status} />
                    </td>
                    <td className="px-4 py-3">
                      <span className="inline-flex items-center gap-1 font-medium whitespace-nowrap text-accent-ink">
                        {typeAction[a.type]} <ArrowRight className="size-4" aria-hidden />
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </>
  )
}
