import { BrainCircuit, Gauge, ListChecks, Zap } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router'
import { useAuth } from '../auth/useAuth'
import { Button } from '../components/ui'

const features = [
  { icon: Gauge, title: 'Qué pasa con cada medidor', text: 'Consumo, voltaje, corriente y factor de potencia frente a su baseline.' },
  { icon: BrainCircuit, title: 'Anomalías explicadas', text: 'La IA separa anomalías reales, explicables, falsos positivos y problemas de datos.' },
  { icon: ListChecks, title: 'Qué atender primero', text: 'Priorización con confianza, evidencia y una acción recomendada.' },
]

export function LoginPage() {
  const { session, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [email, setEmail] = useState('demo@energyai.local')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  if (session) return <Navigate to="/" replace />

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await login(email, password)
      const from = (location.state as { from?: string } | null)?.from
      navigate(from && from !== '/login' ? from : '/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo iniciar sesión')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="grid min-h-full lg:grid-cols-2">
      <div className="hidden flex-col justify-between bg-surface p-12 lg:flex">
        <div className="flex items-center gap-2">
          <span className="flex size-9 items-center justify-center rounded-lg bg-accent text-white">
            <Zap className="size-5" aria-hidden />
          </span>
          <span className="text-lg font-semibold text-ink">EnergyAI</span>
        </div>
        <div className="max-w-md">
          <h1 className="text-3xl font-semibold tracking-tight text-ink">De lecturas eléctricas a decisiones operativas.</h1>
          <p className="mt-3 text-ink-2">Detecta, explica y prioriza anomalías en tus medidores con un motor de IA que muestra su evidencia.</p>
          <ul className="mt-8 space-y-5">
            {features.map(({ icon: Icon, title, text }) => (
              <li key={title} className="flex gap-3">
                <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent">
                  <Icon className="size-4" aria-hidden />
                </span>
                <div>
                  <p className="text-sm font-medium text-ink">{title}</p>
                  <p className="text-sm text-ink-2">{text}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <p className="text-xs text-ink-3">Prueba técnica · AI Energy Management Platform</p>
      </div>

      <div className="flex items-center justify-center p-6">
        <form onSubmit={onSubmit} className="w-full max-w-sm" noValidate>
          <div className="mb-8 flex items-center gap-2 lg:hidden">
            <span className="flex size-8 items-center justify-center rounded-lg bg-accent text-white">
              <Zap className="size-4" aria-hidden />
            </span>
            <span className="font-semibold text-ink">EnergyAI</span>
          </div>
          <h2 className="text-xl font-semibold text-ink">Iniciar sesión</h2>
          <p className="mt-1 text-sm text-ink-2">Accede al panel de gestión energética.</p>

          <label className="mt-6 block text-sm font-medium text-ink" htmlFor="email">
            Correo
          </label>
          <input
            id="email"
            type="email"
            autoComplete="username"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="mt-1.5 h-10 w-full rounded-lg border border-line-strong bg-raised px-3 text-sm text-ink outline-none focus:border-accent"
            required
          />
          <label className="mt-4 block text-sm font-medium text-ink" htmlFor="password">
            Contraseña
          </label>
          <input
            id="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1.5 h-10 w-full rounded-lg border border-line-strong bg-raised px-3 text-sm text-ink outline-none focus:border-accent"
            required
          />
          {error && (
            <p role="alert" className="mt-3 rounded-lg bg-critical-soft px-3 py-2 text-sm text-ink">
              {error}
            </p>
          )}
          <Button type="submit" variant="primary" className="mt-6 w-full" loading={loading} disabled={!email || !password}>
            Entrar
          </Button>
          <p className="mt-4 rounded-lg border border-line bg-surface px-3 py-2 text-xs text-ink-2">
            Usuario demo: <span className="font-medium text-ink">demo@energyai.local</span> · contraseña en el README
          </p>
        </form>
      </div>
    </div>
  )
}
