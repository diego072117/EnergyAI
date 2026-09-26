import { createContext, useContext } from 'react'
import type { AnalysisRun } from '../api/types'

export interface AnalysisValue {
  start: () => Promise<void>
  run: AnalysisRun | null
  running: boolean
  starting: boolean
  error: string | null
  open: boolean
  setOpen: (v: boolean) => void
}

export const AnalysisContext = createContext<AnalysisValue | null>(null)

export function useAnalysis(): AnalysisValue {
  const ctx = useContext(AnalysisContext)
  if (!ctx) throw new Error('useAnalysis must be used inside AnalysisProvider')
  return ctx
}
