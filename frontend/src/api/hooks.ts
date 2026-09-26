import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, qs } from './client'
import type {
  AIStatus,
  Anomaly,
  AnomalyDetail,
  AnomalyList,
  AnomalyStatus,
  DashboardSummary,
  MeterDetail,
  MeterSummary,
  ReadingSeries,
} from './types'

export const keys = {
  dashboard: ['dashboard'] as const,
  meters: (f: MeterFilters) => ['meters', f] as const,
  meter: (id: string) => ['meter', id] as const,
  readings: (id: string, p: ReadingsParams) => ['readings', id, p] as const,
  anomalies: (f: AnomalyFilters) => ['anomalies', f] as const,
  anomaly: (id: number) => ['anomaly', id] as const,
  aiStatus: ['ai-status'] as const,
}

export function useDashboard() {
  return useQuery({ queryKey: keys.dashboard, queryFn: () => api<DashboardSummary>('/dashboard/summary') })
}

export interface MeterFilters {
  status?: string
  q?: string
  sort?: string
  order?: string
}

export function useMeters(f: MeterFilters) {
  return useQuery({
    queryKey: keys.meters(f),
    queryFn: () => api<{ items: MeterSummary[]; total: number }>(`/meters${qs({ ...f })}`),
    placeholderData: keepPreviousData,
  })
}

export function useMeter(id: string) {
  return useQuery({ queryKey: keys.meter(id), queryFn: () => api<MeterDetail>(`/meters/${encodeURIComponent(id)}`) })
}

export interface ReadingsParams {
  from?: string
  to?: string
  resolution?: 'hour' | 'day'
}

export function useReadings(id: string, p: ReadingsParams) {
  return useQuery({
    queryKey: keys.readings(id, p),
    queryFn: () => api<ReadingSeries>(`/meters/${encodeURIComponent(id)}/readings${qs({ ...p })}`),
    enabled: !!id,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}

export interface AnomalyFilters {
  type?: string
  severity?: string
  status?: string
  meter_id?: string
}

export function useAnomalies(f: AnomalyFilters, enabled = true) {
  return useQuery({
    queryKey: keys.anomalies(f),
    queryFn: () => api<AnomalyList>(`/anomalies${qs({ ...f })}`),
    enabled,
    placeholderData: keepPreviousData,
  })
}

export function useAnomaly(id: number) {
  return useQuery({ queryKey: keys.anomaly(id), queryFn: () => api<AnomalyDetail>(`/anomalies/${id}`) })
}

export function useAIStatus() {
  return useQuery({ queryKey: keys.aiStatus, queryFn: () => api<AIStatus>('/ai/status'), staleTime: 60_000 })
}

export function useUpdateAnomaly(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { status: AnomalyStatus; note?: string }) =>
      api<Anomaly>(`/anomalies/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
    onSuccess: () => {
      void qc.invalidateQueries()
    },
  })
}
