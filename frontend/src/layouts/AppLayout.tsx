import clsx from 'clsx'
import { Bot, BrainCircuit, Gauge, LayoutDashboard, LogOut, Moon, Play, Sun, Wrench, Zap } from 'lucide-react'
import { Suspense } from 'react'
import { NavLink, Outlet } from 'react-router'
import { useAnalysis } from '../analysis/useAnalysis'
import { useAIStatus, useDashboard } from '../api/hooks'
import { useAuth } from '../auth/useAuth'
import { Button, Skeleton } from '../components/ui'
import { relativeTime } from '../lib/format'
import { useTheme } from '../lib/theme'

const nav = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, end: true },
  { to: '/meters', label: 'Medidores', icon: Gauge, end: false },
  { to: '/anomalies', label: 'Anomalías IA', icon: BrainCircuit, end: false },
]

function AIStatusChip() {
  const { data } = useAIStatus()
  if (!data) return null
  const llm = data.provider !== 'template'
  return (
    <div className="rounded-lg border border-line bg-page px-3 py-2 text-xs" title={data.detail}>
      <div className="flex items-center gap-1.5 font-medium text-ink">
        {llm ? <Bot className="size-3.5 text-accent" aria-hidden /> : <Wrench className="size-3.5 text-ink-3" aria-hidden />}
        {llm ? 'IA generativa' : 'Explicaciones por reglas'}
      </div>
      <div className="mt-0.5 flex items-center gap-1.5 text-ink-3">
        <span className={clsx('size-1.5 rounded-full', data.available ? 'bg-good' : 'bg-serious')} aria-hidden />
        {llm ? `${data.model} · ${data.available ? 'conectado' : 'no disponible'}` : 'Siempre disponible'}
      </div>
    </div>
  )
}

function LastAnalysis() {
  const { data } = useDashboard()
  const run = data?.last_analysis
  if (!run) return <span className="hidden text-xs text-ink-3 md:inline">Sin análisis todavía</span>
  const label = { COMPLETED: 'Completado', FAILED: 'Falló', RUNNING: 'En curso', PENDING: 'En curso' }[run.status]
  return (
    <span className="hidden text-xs text-ink-3 md:inline">
      Último análisis: <span className="text-ink-2">{label}</span> · {relativeTime(run.finished_at ?? run.started_at)}
    </span>
  )
}

export function AppLayout() {
  const { session, logout } = useAuth()
  const { start, running, starting } = useAnalysis()
  const [theme, toggleTheme] = useTheme()

  return (
    <div className="flex min-h-full">
      <aside className="sticky top-0 hidden h-screen w-60 shrink-0 flex-col border-r border-line bg-surface lg:flex">
        <div className="flex items-center gap-2 px-5 py-5">
          <span className="flex size-8 items-center justify-center rounded-lg bg-accent text-white">
            <Zap className="size-4.5" aria-hidden />
          </span>
          <div>
            <div className="text-sm font-semibold text-ink">EnergyAI</div>
            <div className="text-[11px] text-ink-3">Energy Management</div>
          </div>
        </div>
        <nav className="flex-1 space-y-0.5 px-3" aria-label="Principal">
          {nav.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                clsx(
                  'flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                  isActive ? 'bg-accent-soft text-accent-ink' : 'text-ink-2 hover:bg-hover hover:text-ink',
                )
              }
            >
              <Icon className="size-4" aria-hidden />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="space-y-3 p-3">
          <AIStatusChip />
          <div className="flex items-center justify-between gap-2 border-t border-line px-2 pt-3">
            <div className="min-w-0">
              <div className="truncate text-sm font-medium text-ink">{session?.name}</div>
              <div className="truncate text-xs text-ink-3">{session?.email}</div>
            </div>
            <button onClick={logout} className="rounded-md p-1.5 text-ink-3 hover:bg-hover hover:text-ink" aria-label="Cerrar sesión" title="Cerrar sesión">
              <LogOut className="size-4" />
            </button>
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center justify-between gap-3 border-b border-line bg-page/85 px-4 backdrop-blur sm:px-6">
          <nav className="flex items-center gap-1 lg:hidden" aria-label="Principal móvil">
            {nav.map(({ to, label, icon: Icon, end }) => (
              <NavLink
                key={to}
                to={to}
                end={end}
                aria-label={label}
                className={({ isActive }) => clsx('rounded-md p-2', isActive ? 'bg-accent-soft text-accent-ink' : 'text-ink-2')}
              >
                <Icon className="size-4" aria-hidden />
              </NavLink>
            ))}
          </nav>
          <div className="hidden lg:block">
            <LastAnalysis />
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={toggleTheme}
              className="rounded-md p-2 text-ink-2 hover:bg-hover hover:text-ink"
              aria-label={theme === 'dark' ? 'Usar tema claro' : 'Usar tema oscuro'}
            >
              {theme === 'dark' ? <Sun className="size-4" /> : <Moon className="size-4" />}
            </button>
            <Button variant="primary" onClick={() => void start()} loading={starting || running}>
              {!(starting || running) && <Play className="size-4" aria-hidden />}
              {running ? 'Analizando…' : 'Run AI Analysis'}
            </Button>
          </div>
        </header>
        <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 lg:py-8">
          <Suspense fallback={<Skeleton className="h-96" />}>
            <Outlet />
          </Suspense>
        </main>
      </div>
    </div>
  )
}
