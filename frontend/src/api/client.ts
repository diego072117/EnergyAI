const TOKEN_KEY = 'energyai.token'
const BASE = '/api/v1'

export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export const tokenStore = {
  get(): string | null {
    try {
      return localStorage.getItem(TOKEN_KEY)
    } catch {
      return null
    }
  },
  set(token: string | null) {
    try {
      if (token) localStorage.setItem(TOKEN_KEY, token)
      else localStorage.removeItem(TOKEN_KEY)
    } catch {
      /* storage unavailable (private mode) */
    }
  },
}

let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(fn: (() => void) | null) {
  onUnauthorized = fn
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  const token = tokenStore.get()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  let res: Response
  try {
    res = await fetch(BASE + path, { ...init, headers })
  } catch {
    throw new ApiError(0, 'network', 'No se pudo conectar con el servidor')
  }
  if (res.status === 401 && !path.startsWith('/auth/login')) {
    onUnauthorized?.()
  }
  const text = await res.text()
  const body = text ? JSON.parse(text) : null
  if (!res.ok) {
    const err = body?.error ?? {}
    throw new ApiError(res.status, err.code ?? 'error', err.message ?? `Error ${res.status}`)
  }
  return body as T
}

export function qs(params: Record<string, string | number | undefined | null>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') sp.set(k, String(v))
  }
  const s = sp.toString()
  return s ? `?${s}` : ''
}
