import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { anomaly, anomalyDetail, completedRun, dashboard, emptyDashboard, falsePositive, meters } from './fixtures'
import { loggedIn, mockApi, renderApp } from './utils'

beforeAll(() => {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
})

afterEach(() => {
  vi.unstubAllGlobals()
})

const common = {
  'GET /ai/status': { provider: 'ollama', model: 'qwen2.5:7b', available: true },
}

describe('login', () => {
  it('redirects to login and signs in', async () => {
    const api = mockApi({
      ...common,
      'POST /auth/login': { token: 't', expires_at: '', user: { id: 1, email: 'demo@energyai.local', name: 'Operador Demo' } },
      'GET /dashboard/summary': dashboard,
    })
    renderApp('/')
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Contraseña'), 'demo1234')
    await user.click(screen.getByRole('button', { name: 'Entrar' }))

    expect(await screen.findByRole('heading', { name: 'Dashboard' })).toBeInTheDocument()
    expect(api.calls[0]).toEqual({ method: 'POST', path: '/auth/login', body: { email: 'demo@energyai.local', password: 'demo1234' } })
    expect(localStorage.getItem('energyai.token')).toBe('t')
  })

  it('shows API errors', async () => {
    mockApi({
      'POST /auth/login': () =>
        new Response(JSON.stringify({ error: { code: 'invalid_credentials', message: 'correo o contraseña incorrectos' } }), { status: 401 }),
    })
    renderApp('/login')
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Contraseña'), 'mala')
    await user.click(screen.getByRole('button', { name: 'Entrar' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('correo o contraseña incorrectos')
  })

  it('logs out when the token expires', async () => {
    loggedIn()
    mockApi({ ...common, 'GET /dashboard/summary': () => new Response(JSON.stringify({ error: { code: 'unauthorized', message: 'x' } }), { status: 401 }) })
    renderApp('/')
    expect(await screen.findByRole('button', { name: 'Entrar' })).toBeInTheDocument()
    expect(localStorage.getItem('energyai.token')).toBeNull()
  })
})

describe('dashboard', () => {
  it('shows KPIs and the priority list', async () => {
    loggedIn()
    mockApi({ ...common, 'GET /dashboard/summary': dashboard })
    renderApp('/')
    expect(await screen.findByText('155,3 MWh')).toBeInTheDocument()
    expect(screen.getAllByText('4 anomalías detectadas · 2 requieren atención prioritaria').length).toBeGreaterThan(0)
    const link = screen.getByRole('link', { name: /M-109/ })
    expect(link).toHaveAttribute('href', '/anomalies/1')
    expect(within(link).getByText('Alta')).toBeInTheDocument()
    expect(screen.getByText('qwen2.5:7b · conectado')).toBeInTheDocument()
  })

  it('invites to run the first analysis', async () => {
    loggedIn()
    mockApi({ ...common, 'GET /dashboard/summary': emptyDashboard })
    renderApp('/')
    expect(await screen.findByText('Aún no hay análisis de IA')).toBeInTheDocument()
  })
})

describe('meters', () => {
  it('filters and sorts through the API', async () => {
    loggedIn()
    const api = mockApi({ ...common, 'GET /dashboard/summary': dashboard, 'GET /meters': { items: meters, total: 2 } })
    renderApp('/meters')
    expect(await screen.findByText('M-109')).toBeInTheDocument()
    expect(screen.getByText('+110,7%')).toBeInTheDocument()

    const user = userEvent.setup()
    await user.click(screen.getByRole('radio', { name: 'Críticos' }))
    await waitFor(() => expect(api.calls.some((c) => c.path.startsWith('/meters?') && c.path.includes('status=critical'))).toBe(true))

    await user.click(screen.getByRole('button', { name: /Variación/ }))
    await waitFor(() => expect(api.calls.some((c) => c.path.includes('sort=variation') && c.path.includes('order=desc'))).toBe(true))
  })
})

describe('AI analysis', () => {
  it('runs the analysis and shows the steps and summary', async () => {
    loggedIn()
    const api = mockApi({
      ...common,
      'GET /dashboard/summary': emptyDashboard,
      'POST /ai/analyze': { ...completedRun, status: 'PENDING', steps: completedRun.steps.map((s) => ({ ...s, state: 'PENDING', detail: undefined })), summary: null },
      'GET /ai/analysis/run-1': completedRun,
      'GET /anomalies': { analysis_id: 'run-1', items: [anomaly, falsePositive], total: 2 },
    })
    renderApp('/')
    const user = userEvent.setup()
    await user.click((await screen.findAllByRole('button', { name: /Run AI Analysis/ }))[0])

    const dialog = await screen.findByRole('dialog', { name: 'Análisis de IA' })
    expect(await within(dialog).findByText('4 anomalías detectadas · 2 requieren atención prioritaria')).toBeInTheDocument()
    expect(within(dialog).getByText('detalle Explicación')).toBeInTheDocument()
    expect(within(dialog).getByText('IA generativa · qwen2.5:7b')).toBeInTheDocument()
    await user.click(await within(dialog).findByRole('button', { name: /Investigar M-109/ }))
    expect(api.calls.some((c) => c.method === 'POST' && c.path === '/ai/analyze')).toBe(true)
  })

  it('follows the running analysis on conflict', async () => {
    loggedIn()
    mockApi({
      ...common,
      'GET /dashboard/summary': emptyDashboard,
      'POST /ai/analyze': () => new Response(JSON.stringify({ error: { code: 'analysis_running', message: 'ya hay un análisis en ejecución' } }), { status: 409 }),
      'GET /ai/analysis/latest': completedRun,
      'GET /ai/analysis/run-1': completedRun,
      'GET /anomalies': { analysis_id: 'run-1', items: [anomaly], total: 1 },
    })
    renderApp('/')
    const user = userEvent.setup()
    const buttons = await screen.findAllByRole('button', { name: /Run AI Analysis/ })
    await user.click(buttons[0])
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(/4 anomalías detectadas/)).toBeInTheDocument()
  })
})

describe('investigation', () => {
  it('explains the finding and records the operator action', async () => {
    loggedIn()
    const api = mockApi({
      ...common,
      'GET /dashboard/summary': dashboard,
      'GET /anomalies/1': anomalyDetail,
      'GET /meters/M-109/readings': { meter_id: 'M-109', resolution: 'hour', points: [], baseline: null, thresholds: { shift_pct: 25, nominal_voltage: 220, voltage_tolerance_pct: 5 } },
      'PATCH /anomalies/1': { ...anomaly, status: 'INVESTIGATING', note: 'Cuadrilla enviada' },
    })
    renderApp('/anomalies/1')

    expect(await screen.findByText('Prioridad #1 de 4')).toBeInTheDocument()
    expect(screen.getByText(anomaly.reason)).toBeInTheDocument()
    expect(screen.getByText(anomaly.recommended_action)).toBeInTheDocument()
    expect(screen.getByText('Registro sin evento operativo reportado: no explica el cambio.')).toBeInTheDocument()
    expect(screen.getByText('2 de 2 señales respaldan la clasificación')).toBeInTheDocument()

    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Nota del operador'), 'Cuadrilla enviada')
    await user.click(screen.getByRole('button', { name: /Marcar en investigación/ }))
    await waitFor(() =>
      expect(api.calls).toContainEqual({ method: 'PATCH', path: '/anomalies/1', body: { status: 'INVESTIGATING', note: 'Cuadrilla enviada' } }),
    )
  })

  it('shows an error state when the anomaly does not exist', async () => {
    loggedIn()
    mockApi({ ...common, 'GET /dashboard/summary': dashboard })
    renderApp('/anomalies/99')
    expect(await screen.findByText('No se pudieron cargar los datos')).toBeInTheDocument()
  })
})
