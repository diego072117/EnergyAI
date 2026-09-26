import clsx from 'clsx'
import { ArrowDown, ArrowUp, ArrowUpDown, Gauge, Search } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { useMeters } from '../api/hooks'
import type { MeterSummary } from '../api/types'
import { MeterStatusBadge, SeverityBadge, TypeBadge } from '../components/badges'
import { Card, EmptyState, ErrorState, PageHeader, Segmented, Skeleton } from '../components/ui'
import { num, pct } from '../lib/format'

type Status = 'all' | 'ok' | 'alert' | 'critical'
type SortKey = 'meter' | 'consumption' | 'variation' | 'severity'

const statusOptions: { value: Status; label: string }[] = [
  { value: 'all', label: 'Todos' },
  { value: 'ok', label: 'Normales' },
  { value: 'alert', label: 'Alertas' },
  { value: 'critical', label: 'Críticos' },
]

function Variation({ value }: { value: number }) {
  const big = Math.abs(value) >= 25
  const Icon = value > 0.5 ? ArrowUp : value < -0.5 ? ArrowDown : null
  return (
    <span className="tabular inline-flex items-center justify-end gap-1 text-ink">
      {Icon && <Icon className={clsx('size-3.5', big ? 'text-serious' : 'text-ink-3')} aria-hidden />}
      <span className={big ? 'font-semibold' : undefined}>{pct(value)}</span>
    </span>
  )
}

function SortHeader({
  label,
  k,
  sort,
  order,
  onSort,
  align = 'left',
}: {
  label: string
  k: SortKey
  sort: SortKey
  order: 'asc' | 'desc'
  onSort: (k: SortKey) => void
  align?: 'left' | 'right'
}) {
  const active = sort === k
  const Icon = !active ? ArrowUpDown : order === 'desc' ? ArrowDown : ArrowUp
  return (
    <th scope="col" className={clsx('px-4 py-2.5 font-medium', align === 'right' && 'text-right')} aria-sort={active ? (order === 'desc' ? 'descending' : 'ascending') : 'none'}>
      <button onClick={() => onSort(k)} className={clsx('inline-flex items-center gap-1 hover:text-ink', active && 'text-ink')}>
        {label}
        <Icon className="size-3" aria-hidden />
      </button>
    </th>
  )
}

export function MetersPage() {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const status = (params.get('status') as Status) || 'all'
  const sort = (params.get('sort') as SortKey) || 'severity'
  const order = (params.get('order') as 'asc' | 'desc') || (sort === 'meter' ? 'asc' : 'desc')
  const [q, setQ] = useState(params.get('q') ?? '')
  const deferredQ = useDeferredValue(q)
  const { data, isLoading, error, refetch, isPlaceholderData } = useMeters({ status, q: deferredQ, sort, order })

  const update = (next: Record<string, string | undefined>) => {
    const p = new URLSearchParams(params)
    for (const [k, v] of Object.entries(next)) {
      if (v) p.set(k, v)
      else p.delete(k)
    }
    setParams(p, { replace: true })
  }
  const onSort = (k: SortKey) => {
    const nextOrder = sort === k ? (order === 'desc' ? 'asc' : 'desc') : k === 'meter' ? 'asc' : 'desc'
    update({ sort: k, order: nextOrder })
  }

  return (
    <>
      <PageHeader title="Medidores" subtitle="Consumo de las últimas 24 h frente al baseline diario de cada medidor" />

      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <Segmented label="Filtrar por estado" value={status} onChange={(v) => update({ status: v === 'all' ? undefined : v })} options={statusOptions} />
        <div className="relative w-full sm:w-72">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-3" aria-hidden />
          <input
            type="search"
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              update({ q: e.target.value || undefined })
            }}
            placeholder="Buscar por meter_id, nombre o ubicación"
            aria-label="Buscar medidor"
            className="h-9 w-full rounded-lg border border-line-strong bg-raised pr-3 pl-9 text-sm text-ink outline-none placeholder:text-ink-3 focus:border-accent"
          />
        </div>
      </div>

      <Card className="overflow-hidden">
        {error ? (
          <ErrorState error={error} onRetry={() => void refetch()} />
        ) : isLoading || !data ? (
          <div className="space-y-2 p-4">
            {Array.from({ length: 8 }, (_, i) => (
              <Skeleton key={i} className="h-10" />
            ))}
          </div>
        ) : data.items.length === 0 ? (
          <EmptyState icon={<Gauge className="size-5" />} title="Sin medidores para este filtro" description="Prueba con otro estado o término de búsqueda." />
        ) : (
          <div className={clsx('overflow-x-auto transition-opacity', isPlaceholderData && 'opacity-60')}>
            <table className="w-full min-w-[760px] text-sm">
              <thead className="border-b border-line text-left text-xs text-ink-3">
                <tr>
                  <SortHeader label="Medidor" k="meter" sort={sort} order={order} onSort={onSort} />
                  <SortHeader label="Consumo 24 h" k="consumption" sort={sort} order={order} onSort={onSort} align="right" />
                  <th scope="col" className="px-4 py-2.5 text-right font-medium">
                    Baseline diario
                  </th>
                  <SortHeader label="Variación" k="variation" sort={sort} order={order} onSort={onSort} align="right" />
                  <SortHeader label="Estado" k="severity" sort={sort} order={order} onSort={onSort} />
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Anomalía IA
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {data.items.map((m: MeterSummary) => (
                  <tr
                    key={m.meter_id}
                    onClick={() => navigate(`/meters/${m.meter_id}`)}
                    onKeyDown={(e) => e.key === 'Enter' && navigate(`/meters/${m.meter_id}`)}
                    tabIndex={0}
                    className="cursor-pointer hover:bg-hover focus-visible:bg-hover"
                  >
                    <td className="px-4 py-3">
                      <div className="font-semibold text-ink">{m.meter_id}</div>
                      <div className="text-xs text-ink-3">
                        {m.name} · {m.location}
                      </div>
                    </td>
                    <td className="tabular px-4 py-3 text-right text-ink">{num(m.last_24h_kwh)} kWh</td>
                    <td className="tabular px-4 py-3 text-right text-ink-2">{num(m.baseline_daily_kwh)} kWh</td>
                    <td className="px-4 py-3 text-right">
                      <Variation value={m.variation_pct} />
                    </td>
                    <td className="px-4 py-3">
                      <MeterStatusBadge status={m.status} />
                    </td>
                    <td className="px-4 py-3">
                      {m.anomaly ? (
                        <div className="flex flex-wrap items-center gap-1.5">
                          <TypeBadge type={m.anomaly.type} />
                          {m.anomaly.anomaly && <SeverityBadge severity={m.anomaly.severity} />}
                        </div>
                      ) : (
                        <span className="text-ink-3">—</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      {data && <p className="mt-3 text-xs text-ink-3">{data.total} medidores</p>}
    </>
  )
}
