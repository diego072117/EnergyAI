import { QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { vi } from 'vitest'
import { createQueryClient } from '../api/queryClient'
import { AppRoutes } from '../routes'
import { AuthProvider } from '../auth/auth'

type Handler = (req: { method: string; path: string; url: URL; body: unknown }) => unknown | Response

export interface MockApi {
  calls: { method: string; path: string; body: unknown }[]
}

export function mockApi(routes: Record<string, Handler | unknown>): MockApi {
  const api: MockApi = { calls: [] }
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(String(input), 'http://localhost')
      const method = (init?.method ?? 'GET').toUpperCase()
      const path = url.pathname.replace(/^\/api\/v1/, '')
      const body = init?.body ? JSON.parse(String(init.body)) : undefined
      api.calls.push({ method, path: path + url.search, body })
      const route = routes[`${method} ${path}`]
      if (route === undefined) {
        return new Response(JSON.stringify({ error: { code: 'not_found', message: `sin mock para ${method} ${path}` } }), { status: 404 })
      }
      const result = typeof route === 'function' ? (route as Handler)({ method, path, url, body }) : route
      if (result instanceof Response) return result
      return new Response(JSON.stringify(result), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }),
  )
  return api
}

export function loggedIn() {
  localStorage.setItem('energyai.token', 'test-token')
  localStorage.setItem('energyai.session', JSON.stringify({ email: 'demo@energyai.local', name: 'Operador Demo' }))
}

export function renderApp(route: string) {
  const client = createQueryClient()
  client.setDefaultOptions({ queries: { retry: false, staleTime: 0 } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[route]}>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}
