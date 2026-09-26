import { Compass } from 'lucide-react'
import { lazy, type ComponentType } from 'react'
import { Link, Route, Routes } from 'react-router'
import { AnalysisProvider } from './analysis/analysis'
import { RequireAuth } from './auth/auth'
import { EmptyState } from './components/ui'
import { AppLayout } from './layouts/AppLayout'
import { LoginPage } from './pages/LoginPage'

const page = <K extends string>(load: () => Promise<Record<K, ComponentType>>, name: K) =>
  lazy(() => load().then((m) => ({ default: m[name] })))

const DashboardPage = page(() => import('./pages/DashboardPage'), 'DashboardPage')
const MetersPage = page(() => import('./pages/MetersPage'), 'MetersPage')
const MeterDetailPage = page(() => import('./pages/MeterDetailPage'), 'MeterDetailPage')
const AnomaliesPage = page(() => import('./pages/AnomaliesPage'), 'AnomaliesPage')
const InvestigationPage = page(() => import('./pages/InvestigationPage'), 'InvestigationPage')

function NotFound() {
  return (
    <EmptyState
      icon={<Compass className="size-5" />}
      title="Página no encontrada"
      action={
        <Link to="/" className="text-sm font-medium text-accent-ink hover:underline">
          Volver al dashboard
        </Link>
      }
    />
  )
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <RequireAuth>
            <AnalysisProvider>
              <AppLayout />
            </AnalysisProvider>
          </RequireAuth>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="meters" element={<MetersPage />} />
        <Route path="meters/:meterId" element={<MeterDetailPage />} />
        <Route path="anomalies" element={<AnomaliesPage />} />
        <Route path="anomalies/:id" element={<InvestigationPage />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  )
}
