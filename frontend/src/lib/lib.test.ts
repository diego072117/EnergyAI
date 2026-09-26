import { describe, expect, it } from 'vitest'
import { confidence, dateTime, day, energy, num, pct, period, relativeTime } from './format'
import { confidenceLevel, explanationSource } from './labels'

describe('format', () => {
  it('formats numbers in es-CO', () => {
    expect(num(2208.4)).toBe('2.208')
    expect(num(0.736, 2)).toBe('0,74')
  })

  it('formats signed percentages', () => {
    expect(pct(110.44)).toBe('+110,4%')
    expect(pct(-37.04)).toBe('−37,0%')
    expect(pct(0.01)).toBe('0,0%')
  })

  it('picks the energy unit', () => {
    expect(energy(820)).toBe('820 kWh')
    expect(energy(155250)).toBe('155,3 MWh')
  })

  it('renders dates in UTC like the dataset', () => {
    expect(dateTime('2026-09-12T14:00:00Z')).toBe('12-sep 14:00')
    expect(day('2026-09-01')).toBe('1-sep')
    expect(period('2026-09-01T00:00:00Z', '2026-09-14T23:00:00Z')).toBe('1–14 sep 2026')
  })

  it('formats confidence and relative time', () => {
    expect(confidence(0.976)).toBe('98%')
    const now = new Date('2026-09-26T10:10:00Z')
    expect(relativeTime('2026-09-26T10:09:30Z', now)).toBe('hace unos segundos')
    expect(relativeTime('2026-09-26T10:00:00Z', now)).toBe('hace 10 min')
    expect(relativeTime('2026-09-26T07:00:00Z', now)).toBe('hace 3 h')
  })
})

describe('labels', () => {
  it('maps confidence levels', () => {
    expect(confidenceLevel(0.9)).toBe('Alta')
    expect(confidenceLevel(0.7)).toBe('Media')
    expect(confidenceLevel(0.5)).toBe('Baja')
  })

  it('describes the explanation source', () => {
    expect(explanationSource('LLM:qwen2.5:7b')).toEqual({ ai: true, label: 'IA generativa · qwen2.5:7b' })
    expect(explanationSource('TEMPLATE')).toEqual({ ai: false, label: 'Motor de reglas' })
  })
})
