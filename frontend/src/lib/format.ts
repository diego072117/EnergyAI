// Spanish formatting. Dataset timestamps are UTC, so dates are rendered in UTC
// to match the data and the AI explanations exactly.

const LOCALE = 'es-CO'

export function num(v: number, decimals = 0): string {
  return v.toLocaleString(LOCALE, { minimumFractionDigits: decimals, maximumFractionDigits: decimals })
}

export function pct(v: number, decimals = 1): string {
  const s = num(Math.abs(v), decimals)
  if (Math.abs(v) < 0.5 * 10 ** -decimals) return `${num(0, decimals)}%`
  return `${v > 0 ? '+' : '−'}${s}%`
}

export function energy(kwh: number): string {
  if (Math.abs(kwh) >= 10_000) return `${num(kwh / 1000, 1)} MWh`
  return `${num(kwh, 0)} kWh`
}

export function confidence(v: number): string {
  return `${Math.round(v * 100)}%`
}

const MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic']

function pad(n: number) {
  return n.toString().padStart(2, '0')
}

export function dateTime(iso: string | Date): string {
  const d = typeof iso === 'string' ? new Date(iso) : iso
  return `${d.getUTCDate()}-${MONTHS[d.getUTCMonth()]} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`
}

export function day(iso: string | Date): string {
  const d = typeof iso === 'string' ? new Date(iso) : iso
  return `${d.getUTCDate()}-${MONTHS[d.getUTCMonth()]}`
}

export function period(from: string, to: string): string {
  const a = new Date(from)
  const b = new Date(to)
  const sameMonth = a.getUTCMonth() === b.getUTCMonth()
  const left = sameMonth ? `${a.getUTCDate()}` : `${a.getUTCDate()} ${MONTHS[a.getUTCMonth()]}`
  return `${left}–${b.getUTCDate()} ${MONTHS[b.getUTCMonth()]} ${b.getUTCFullYear()}`
}

export function relativeTime(iso: string, now = new Date()): string {
  const diff = Math.round((now.getTime() - new Date(iso).getTime()) / 1000)
  if (diff < 60) return 'hace unos segundos'
  if (diff < 3600) return `hace ${Math.floor(diff / 60)} min`
  if (diff < 86400) return `hace ${Math.floor(diff / 3600)} h`
  return new Date(iso).toLocaleString(LOCALE, { dateStyle: 'medium', timeStyle: 'short' })
}

export function hours(h: number): string {
  return h === 1 ? '1 hora' : `${h} horas`
}
