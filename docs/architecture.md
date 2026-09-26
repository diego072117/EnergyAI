# Arquitectura

## Vista general

```mermaid
flowchart TB
  subgraph Browser
    SPA[React SPA<br/>Login · Dashboard · Medidores · Detalle · Anomalías · Investigación]
  end
  subgraph API[API Go]
    MW[middleware<br/>request id · logs · CORS · JWT]
    H[httpapi handlers]
    S[service<br/>MeterService · AnomalyService · DashboardService · Runner · Auth]
    A[analytics<br/>motor puro]
    X[ai<br/>Template · LLM · WithFallback]
    R[(store · PostgreSQL)]
  end
  O[Ollama local / API OpenAI-compatible]
  SPA -->|/api/v1| MW --> H --> S
  S --> A
  S --> X
  S --> R
  X -.->|opcional| O
```

Capas: `handler → service → repository`. Los handlers no acceden a SQL. Los servicios dependen de la interfaz `service.Repository`, implementada por `store.Store` (PostgreSQL) y `memstore.Store` (tests), así que la API completa se prueba sin base de datos.

## Flujo de Run AI Analysis

```mermaid
sequenceDiagram
  participant U as Usuario
  participant F as Frontend
  participant A as API
  participant R as Runner (goroutine)
  participant DB as PostgreSQL
  participant L as Ollama
  U->>F: Run AI Analysis
  F->>A: POST /ai/analyze
  A->>R: Start (mutex: 1 análisis a la vez)
  A-->>F: 202 {id}
  loop cada 600 ms
    F->>A: GET /ai/analysis/{id}
    A-->>F: pasos y estado
  end
  R->>DB: lecturas y eventos
  R->>R: baseline → detección → correlación → eventos
  R->>L: explicaciones en paralelo (opcional)
  R->>DB: CompleteRun (tx: anomalías + estado de medidores + resumen)
  F->>F: invalida cachés → dashboard y tablas se refrescan
```

- Cada paso se persiste en `analysis_runs.steps` (JSONB) con su estado, detalle y tiempos.
- `ANALYSIS_STEP_DELAY_MS` fija una duración visible mínima por paso para la demo.
- Al arrancar, los análisis que quedaron `RUNNING` por un reinicio se marcan `FAILED`.

## Modelo de datos

```mermaid
erDiagram
  meters ||--o{ readings : tiene
  meters ||--o{ events : tiene
  meters ||--o{ anomalies : tiene
  analysis_runs ||--o{ anomalies : produce
  meters { bigint id text meter_id text name text location text status timestamptz created_at }
  readings { bigint id text meter_id timestamptz timestamp float consumption_kwh float voltage_v float current_a float power_factor text status }
  events { bigint id text meter_id timestamptz timestamp text type text description }
  analysis_runs { text id text status text current_step jsonb steps jsonb summary text error timestamptz started_at timestamptz finished_at }
  anomalies { bigint id text analysis_id text meter_id text type text severity float confidence float priority_score text reason text recommended_action jsonb evidence text status text note }
  users { bigint id text email text name text password_hash }
```

- `anomalies.evidence` (JSONB) guarda todo lo que respalda el hallazgo: variables antes y después, señales, eventos evaluados y el desglose de confianza y prioridad.
- **Estado del medidor**, derivado de sus anomalías activas: `CRITICAL` si hay una real de severidad alta, `ALERT` si hay otra alta o media, `OK` en el resto de casos. Resolver o descartar una anomalía recalcula el estado.
- Las decisiones del operador se conservan entre análisis para el mismo medidor y tipo.

## Seguridad

- JWT HS256 con expiración, emisor validado y algoritmo fijo.
- Contraseñas con bcrypt. El login compara contra un hash ficticio si el usuario no existe, para no filtrar qué correos están registrados.
- Cuerpos JSON limitados a 64 KB y sin campos desconocidos. Los errores internos se registran en el log y no se exponen al cliente.
- La imagen de la API usa `distroless:nonroot`.

## Frontend

- React Router con rutas cargadas bajo demanda. La librería de gráficos va en un chunk aparte.
- TanStack Query para la caché. Un 401 cierra la sesión. Cuando termina un análisis se invalidan todas las consultas.
- Tokens de diseño en CSS (claro y oscuro con selector). Los colores de estado se reservan para la severidad y siempre van con icono y etiqueta.
- Cada gráfico tiene tooltip y **vista de tabla** accesible.
