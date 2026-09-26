import clsx from 'clsx'
import { CircleX, LoaderCircle, RefreshCw } from 'lucide-react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return <section className={clsx('rounded-xl border border-line bg-surface', className)}>{children}</section>
}

export function CardHeader({
  title,
  subtitle,
  action,
  icon,
}: {
  title: ReactNode
  subtitle?: ReactNode
  action?: ReactNode
  icon?: ReactNode
}) {
  return (
    <header className="flex items-start justify-between gap-4 px-5 pt-4 pb-3">
      <div className="min-w-0">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          {icon}
          {title}
        </h2>
        {subtitle && <p className="mt-0.5 text-xs text-ink-3">{subtitle}</p>}
      </div>
      {action}
    </header>
  )
}

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'

export function Button({
  variant = 'secondary',
  size = 'md',
  loading,
  className,
  children,
  disabled,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; size?: 'sm' | 'md'; loading?: boolean }) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      className={clsx(
        'inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-60',
        size === 'sm' ? 'h-8 px-3 text-xs' : 'h-9 px-4 text-sm',
        variant === 'primary' && 'bg-accent text-white hover:brightness-110',
        variant === 'secondary' && 'border border-line-strong bg-raised text-ink hover:bg-hover',
        variant === 'ghost' && 'text-ink-2 hover:bg-hover hover:text-ink',
        variant === 'danger' && 'border border-line-strong bg-raised text-critical hover:bg-critical-soft',
        className,
      )}
    >
      {loading && <LoaderCircle className="size-4 animate-spin" aria-hidden />}
      {children}
    </button>
  )
}

export function PageHeader({ title, subtitle, actions }: { title: ReactNode; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-xl font-semibold tracking-tight text-ink sm:text-2xl">{title}</h1>
        {subtitle && <p className="mt-1 text-sm text-ink-2">{subtitle}</p>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={clsx('animate-pulse-soft rounded-md bg-hover', className)} aria-hidden />
}

export function EmptyState({ icon, title, description, action }: { icon: ReactNode; title: string; description?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center px-6 py-12 text-center">
      <div className="mb-3 flex size-11 items-center justify-center rounded-full bg-accent-soft text-accent">{icon}</div>
      <p className="font-medium text-ink">{title}</p>
      {description && <p className="mt-1 max-w-md text-sm text-ink-2">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const msg = error instanceof Error ? error.message : 'Error inesperado'
  return (
    <div role="alert" className="flex flex-col items-center justify-center px-6 py-10 text-center">
      <CircleX className="mb-2 size-6 text-critical" aria-hidden />
      <p className="font-medium text-ink">No se pudieron cargar los datos</p>
      <p className="mt-1 text-sm text-ink-2">{msg}</p>
      {onRetry && (
        <Button className="mt-4" size="sm" onClick={onRetry}>
          <RefreshCw className="size-3.5" aria-hidden /> Reintentar
        </Button>
      )}
    </div>
  )
}

export function Segmented<T extends string>({
  value,
  onChange,
  options,
  label,
  size = 'md',
}: {
  value: T
  onChange: (v: T) => void
  options: { value: T; label: ReactNode }[]
  label: string
  size?: 'sm' | 'md'
}) {
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex rounded-lg border border-line bg-page p-0.5">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          onClick={() => onChange(o.value)}
          className={clsx(
            'rounded-md font-medium whitespace-nowrap transition-colors',
            size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-sm',
            value === o.value ? 'bg-raised text-ink shadow-sm' : 'text-ink-2 hover:text-ink',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

export function Kpi({
  label,
  value,
  hint,
  icon,
  tone,
}: {
  label: string
  value: ReactNode
  hint?: ReactNode
  icon?: ReactNode
  tone?: 'critical' | 'accent'
}) {
  return (
    <div className="rounded-xl border border-line bg-surface px-4 py-3.5">
      <div className="flex items-center justify-between gap-2 text-xs font-medium text-ink-2">
        <span>{label}</span>
        {icon && <span className={clsx(tone === 'critical' ? 'text-critical' : 'text-ink-3')}>{icon}</span>}
      </div>
      <div className="mt-1.5 text-2xl font-semibold tracking-tight text-ink">{value}</div>
      {hint && <div className="mt-0.5 text-xs text-ink-3">{hint}</div>}
    </div>
  )
}
