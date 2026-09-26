import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { api, ApiError } from '../api/client'
import type { AnalysisRun } from '../api/types'
import { AnalysisDialog } from './AnalysisDialog'
import { AnalysisContext } from './useAnalysis'

const POLL_MS = 600

export function AnalysisProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient()
  const [runId, setRunId] = useState<string | null>(null)
  const [open, setOpen] = useState(false)
  const [starting, setStarting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const finished = useRef<string | null>(null)

  const { data: run = null } = useQuery({
    queryKey: ['analysis', runId],
    queryFn: () => api<AnalysisRun>(`/ai/analysis/${runId}`),
    enabled: !!runId,
    refetchInterval: (q) => {
      const s = q.state.data?.status
      return s === 'COMPLETED' || s === 'FAILED' ? false : POLL_MS
    },
  })

  useEffect(() => {
    if (run && (run.status === 'COMPLETED' || run.status === 'FAILED') && finished.current !== run.id) {
      finished.current = run.id
      void qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'analysis' })
    }
  }, [run, qc])

  const start = useCallback(async () => {
    setError(null)
    setStarting(true)
    setOpen(true)
    try {
      const r = await api<AnalysisRun>('/ai/analyze', { method: 'POST' })
      qc.setQueryData(['analysis', r.id], r)
      setRunId(r.id)
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        // Another analysis is running: follow it instead of failing.
        const latest = await api<AnalysisRun>('/ai/analysis/latest')
        setRunId(latest.id)
      } else {
        setError(e instanceof Error ? e.message : 'No se pudo iniciar el análisis')
      }
    } finally {
      setStarting(false)
    }
  }, [qc])

  const running = !!run && (run.status === 'PENDING' || run.status === 'RUNNING')
  const value = useMemo(
    () => ({ start, run, running, starting, error, open, setOpen }),
    [start, run, running, starting, error, open],
  )
  return (
    <AnalysisContext.Provider value={value}>
      {children}
      <AnalysisDialog />
    </AnalysisContext.Provider>
  )
}
