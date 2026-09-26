# EnergyAI · AI Energy Management Platform

MVP de gestión energética que convierte lecturas de medidores eléctricos en **decisiones operativas**: detecta anomalías, las **explica con evidencia**, distingue las reales de las explicables, los falsos positivos y los problemas de calidad de datos, las **prioriza** y **recomienda una acción**.

```
DATOS → ANÁLISIS → ANOMALÍA → EXPLICACIÓN → PRIORIZACIÓN → ACCIÓN
```

| Capa | Tecnología |
|---|---|
| Backend | Go 1.27 · chi · pgx · JWT |
| Base de datos | PostgreSQL 17 (migraciones y carga de datos automáticas) |
| Motor de anomalías | Go puro: estadística robusta + reglas físicas + correlación con eventos |
| IA generativa (opcional) | Ollama local (`qwen2.5:7b`) o cualquier API compatible con OpenAI, con respaldo automático a plantillas |
| Frontend | React 19 · TypeScript · Vite · Tailwind 4 · TanStack Query · Recharts |
| Calidad | 66 tests en Go (unitarios, integración con PostgreSQL y API) · 17 tests en React · CI en GitHub Actions |

---

## Contenido

1. [Resultado sobre el dataset](#1-resultado-sobre-el-dataset)
2. [Requisitos](#2-requisitos)
3. [Clonar el proyecto](#3-clonar-el-proyecto)
4. [Opción A · Todo con Docker (recomendada)](#4-opción-a--todo-con-docker-recomendada)
5. [Opción B · Instalación manual paso a paso](#5-opción-b--instalación-manual-paso-a-paso)
6. [IA generativa con Ollama (opcional)](#6-ia-generativa-con-ollama-opcional)
7. [Uso de la plataforma](#7-uso-de-la-plataforma)
8. [Tests](#8-tests)
9. [Arquitectura](#9-arquitectura)
10. [API](#10-api)
11. [Solución de problemas](#11-solución-de-problemas)
12. [Decisiones y límites conocidos](#12-decisiones-y-límites-conocidos)

---

## 1. Resultado sobre el dataset

| # | Medidor | Tipo | Severidad | Confianza | Prioridad | Acción |
|---|---|---|---|---|---|---|
| 1 | **M-109** | Anomalía real | Alta | 98% | 97/100 | Investigar |
| 2 | M-112 | Calidad de datos | Alta | 92% | 87/100 | Validar medidor |
| 3 | M-104 | Anomalía explicable | Media | 87% | 48/100 | Validar operación |
| 4 | M-106 | Falso positivo | Baja | 95% | 15/100 | No escalar |

**4 anomalías detectadas · 2 requieren atención prioritaria.** Los otros 8 medidores no generan ninguna alerta.
El motor **no conoce los IDs de los medidores** ni usa `expected_results.csv`: las reglas son genéricas y los IDs solo aparecen en los tests.

---

## 2. Requisitos

| Herramienta | Versión | Necesaria para | Verificar |
|---|---|---|---|
| Git | cualquiera | clonar el repo | `git --version` |
| Docker Desktop | 4.x+ | Opción A, o solo para PostgreSQL en la Opción B | `docker --version` y `docker compose version` |
| Go | **1.27+** | Opción B (backend) | `go version` |
| Node.js | **22+** (incluye npm) | Opción B (frontend) | `node --version` y `npm --version` |
| PostgreSQL | 17 (16 también sirve) | Opción B sin Docker | `psql --version` |
| Ollama | 0.x | Opcional: explicaciones con un LLM local | `ollama --version` |

### Instalación rápida de los requisitos

**Windows** (PowerShell, con `winget`):

```powershell
winget install -e --id Git.Git
winget install -e --id Docker.DockerDesktop
winget install -e --id GoLang.Go
winget install -e --id OpenJS.NodeJS.LTS
winget install -e --id PostgreSQL.PostgreSQL.17   # solo si no vas a usar Docker para la base de datos
winget install -e --id Ollama.Ollama              # opcional
```

> Después de instalar, **cierra y vuelve a abrir la terminal** para que `go`, `node` y `psql` queden en el `PATH`.

**macOS** (Homebrew):

```bash
brew install git go node postgresql@17 ollama
brew install --cask docker
```

**Linux**: instala Docker Engine + Compose plugin (https://docs.docker.com/engine/install/), Go desde https://go.dev/dl/, Node 22 desde https://nodejs.org o con `nvm`, y PostgreSQL con el gestor de paquetes de tu distribución.

---

## 3. Clonar el proyecto

```bash
git clone <URL_DEL_REPOSITORIO> energyai
cd energyai
```

Estructura principal:

```
energyai/
├── backend/            API en Go, motor de anomalías y datos (backend/data/*.csv)
├── frontend/           SPA en React + TypeScript
├── docs/               arquitectura, motor de anomalías y guion de la demo
├── docker-compose.yml  PostgreSQL + API + frontend
└── .env.example        variables para docker compose
```

---

## 4. Opción A · Todo con Docker (recomendada)

Levanta PostgreSQL, la API y el frontend con un solo comando. **No necesitas Go, Node ni PostgreSQL instalados**, solo Docker Desktop abierto.

```bash
docker compose up -d --build
```

La primera vez tarda unos minutos (descarga imágenes y compila). Al terminar:

| Servicio | URL / puerto |
|---|---|
| **Aplicación web** | **http://localhost:8081** |
| API REST | http://localhost:8080 (`/health` para comprobar) |
| PostgreSQL | `localhost:5432` · base `energyai` · usuario `energy` · contraseña `energy` |

**Credenciales de la demo:** `demo@energyai.local` / `demo1234`

Al arrancar, la API **crea las tablas y carga automáticamente** los 12 medidores, las 4.032 lecturas y los 4 eventos de `backend/data/`. No hay que ejecutar ningún SQL.

Comandos útiles:

```bash
docker compose ps                 # estado de los servicios
docker compose logs -f api        # logs de la API
docker compose down               # detener (conserva los datos)
docker compose down -v            # detener y BORRAR la base de datos (se recarga al volver a levantar)
```

Para cambiar variables (proveedor de IA, contraseña demo, etc.), copia `.env.example` a `.env` en la raíz y edítalo antes de `docker compose up`:

```bash
cp .env.example .env              # PowerShell: Copy-Item .env.example .env
```

---

## 5. Opción B · Instalación manual paso a paso

Para desarrollo: base de datos, backend y frontend corriendo por separado, con recarga en caliente del frontend.

### 5.1 Base de datos (PostgreSQL)

Elige **una** de las dos alternativas.

#### Alternativa 1 · PostgreSQL con Docker (más simple)

```bash
docker compose up -d db
```

Crea la base `energyai` con usuario `energy` y contraseña `energy` en `localhost:5432`. Los datos persisten en el volumen `energyai_pgdata`.

#### Alternativa 2 · PostgreSQL instalado localmente

Con PostgreSQL instalado y en ejecución, crea el usuario y la base de datos. En Windows usa la contraseña del usuario `postgres` que definiste al instalar.

```bash
psql -U postgres -c "CREATE USER energy WITH PASSWORD 'energy';"
psql -U postgres -c "CREATE DATABASE energyai OWNER energy;"
```

> Si usas otro usuario, contraseña, host o puerto, ajusta `DATABASE_URL` en `backend/.env` (paso 5.2).

#### Esquema y datos

No hay que ejecutar scripts: al iniciar, el backend

1. aplica las migraciones de `backend/internal/store/migrations/` (tabla de control `schema_migrations`),
2. carga `readings.csv`, `events.csv` y `meters.json` si la base está vacía,
3. crea o actualiza el usuario demo.

Tablas: `meters`, `readings`, `events`, `analysis_runs`, `anomalies`, `users`.

#### Conectarse con DBeaver (u otro cliente)

Nueva conexión → PostgreSQL → Host `localhost` · Puerto `5432` · Base de datos `energyai` · Usuario `energy` · Contraseña `energy`.
La evidencia de cada anomalía está en `anomalies.evidence` (JSONB) y el progreso de cada análisis en `analysis_runs.steps`.

### 5.2 Backend (Go)

```bash
cd backend
cp .env.example .env              # PowerShell: Copy-Item .env.example .env
go mod download
go run ./cmd/api
```

Salida esperada:

```
level=INFO msg="database seeded" meters=12 readings=4032 events=4
level=INFO msg="API listening" addr=:8080 llm_provider=none
```

Comprobar:

```bash
curl http://localhost:8080/health
```

→ `{"status":"ok"}`

> Por defecto `LLM_PROVIDER=none`: las explicaciones se generan con plantillas y no hace falta nada más. Para usar un LLM local, ve a la sección 6.

Para compilar un binario:

```bash
go build -o energyai-api ./cmd/api
```

#### Variables de entorno del backend

Se leen del entorno o de `backend/.env`. Las variables ya definidas en el entorno tienen prioridad sobre el archivo.

| Variable | Por defecto | Descripción |
|---|---|---|
| `PORT` | `8080` | Puerto HTTP de la API |
| `DATABASE_URL` | `postgres://energy:energy@localhost:5432/energyai?sslmode=disable` | Conexión a PostgreSQL (la API reintenta hasta 60 s mientras arranca) |
| `DATA_DIR` | `./data` | Carpeta con `readings.csv`, `events.csv` y `meters.json` |
| `JWT_SECRET` | `dev-secret-change-me` | Clave de firma de los tokens (mínimo 8 caracteres; **cámbiala fuera de la demo**) |
| `JWT_TTL` | `12h` | Duración de la sesión |
| `DEMO_EMAIL` / `DEMO_PASSWORD` / `DEMO_NAME` | `demo@energyai.local` / `demo1234` / `Operador Demo` | Usuario creado al arrancar |
| `CORS_ORIGINS` | `http://localhost:5173` | Orígenes permitidos, separados por coma |
| `ANALYSIS_STEP_DELAY_MS` | `450` | Duración visible mínima de cada paso del análisis (`0` = sin espera) |
| `BASELINE_DAYS` | `7` | Días de referencia para el baseline |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LLM_PROVIDER` | `none` | `none` (plantillas) · `ollama` · `openai` |
| `LLM_BASE_URL` | `http://localhost:11434` (ollama) · `https://api.openai.com/v1` (openai) | URL del proveedor |
| `LLM_MODEL` | `qwen2.5:7b` | Modelo a usar |
| `LLM_API_KEY` | — | Solo para `openai` o compatibles |
| `LLM_TIMEOUT` | `90s` | Tiempo máximo por explicación |

### 5.3 Frontend (React)

En otra terminal:

```bash
cd frontend
npm install
npm run dev
```

Abre **http://localhost:5173**. Vite redirige `/api` a `http://localhost:8080`, así que el backend debe estar corriendo.

| Script | Qué hace |
|---|---|
| `npm run dev` | Servidor de desarrollo con recarga en caliente |
| `npm run build` | Typecheck + build de producción en `dist/` |
| `npm run preview` | Sirve el build de producción localmente |
| `npm test` | Tests (Vitest + Testing Library) |
| `npm run lint` | Linter (oxlint) |
| `npm run typecheck` | Solo verificación de tipos |

---

## 6. IA generativa con Ollama (opcional)

La plataforma **funciona completa sin ningún LLM ni API key**: la detección, la clasificación, la confianza y la prioridad las calcula el motor, y las explicaciones se generan con plantillas a partir de la evidencia. Con Ollama, un modelo **local y gratuito** redacta la explicación y la recomendación.

1. Instala Ollama (sección 2) y descarga el modelo (~4,7 GB):

   ```bash
   ollama pull qwen2.5:7b
   ```

2. Activa el proveedor:
   - **Opción B (manual):** en `backend/.env` pon `LLM_PROVIDER=ollama` y reinicia el backend.
   - **Opción A (Docker):** en el `.env` de la raíz pon `LLM_PROVIDER=ollama` y ejecuta `docker compose up -d`. El contenedor llega a Ollama por `host.docker.internal:11434`.

3. Verifica: la barra lateral de la app muestra **"IA generativa · qwen2.5:7b · conectado"**, o bien:

   ```bash
   curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/ai/status
   ```

Con una GPU NVIDIA el modelo corre en la GPU (unos 2 s por explicación); sin GPU también funciona, pero más lento.

La solución se probó de punta a punta con un **LLM local** (Ollama + `qwen2.5:7b`), sin API keys ni servicios externos.

Otros proveedores: `LLM_PROVIDER=openai` con `LLM_BASE_URL`, `LLM_MODEL` y `LLM_API_KEY` sirve para OpenAI, Groq, OpenRouter, Gemini o cualquier API compatible.

**Qué hace y qué no hace el LLM:** solo redacta `reason` y `recommended_action` a partir de la evidencia ya calculada. **Nunca** decide el tipo, la severidad, la confianza ni la prioridad. Su salida se valida (esquema JSON, longitud y porcentajes que existan en la evidencia). Si falla, se usan las plantillas. La etiqueta de cada explicación indica su origen: *IA generativa · modelo* o *Motor de reglas*.

Los modelos quedan en `~/.ollama/models` (Windows: `C:\Users\<usuario>\.ollama\models`). Para borrar el modelo:

```bash
ollama rm qwen2.5:7b
```

---

## 7. Uso de la plataforma

Flujo de la demo: **Login → Dashboard → Run AI Analysis → M-109 → Investigación → Acción**.

1. **Dashboard:** KPIs (medidores, consumo, anomalías, alta prioridad, confianza, último análisis), la lista "Requiere atención" y el consumo diario de la flota.
2. **Run AI Analysis** (botón superior): ejecuta el pipeline y muestra en vivo los 7 pasos (Lecturas → Baseline → Detección → Correlación → Eventos → Explicación → Recomendación).
3. **Medidores:** filtros (todos, normales, alertas, críticos), búsqueda por `meter_id`, nombre o ubicación, y orden por consumo, variación o severidad.
4. **Detalle de medidor:** consumo actual frente al baseline, variación, histórico de consumo, voltaje, corriente y factor de potencia con la banda esperada, la ventana de la anomalía y los eventos.
5. **Anomalías IA:** hallazgos ordenados por prioridad, con tipo, severidad, confianza y acción.
6. **Investigación:** qué encontró la IA, comparación contra el baseline, variables que cambiaron, evidencia, eventos relacionados, desglose de confianza y prioridad, y acciones del operador (en investigación, resolver, descartar, con nota).

Guion detallado de 7 minutos: [`docs/demo-script.md`](docs/demo-script.md).

---

## 8. Tests

### Backend

```bash
cd backend
go vet ./...
go test ./...
```

Los tests del repositorio PostgreSQL (`internal/store`) se **saltan** si no se define `TEST_DATABASE_URL`. Para ejecutarlos, crea una base de datos **exclusiva para tests**. Los tests vacían sus tablas, así que el nombre debe contener `test`:

```bash
docker compose exec db psql -U energy -d energyai -c "CREATE DATABASE energyai_test"
```

```bash
TEST_DATABASE_URL="postgres://energy:energy@localhost:5432/energyai_test?sslmode=disable" go test ./...
```

En PowerShell:

```powershell
$env:TEST_DATABASE_URL="postgres://energy:energy@localhost:5432/energyai_test?sslmode=disable"; go test ./...
```

Prueba en vivo contra un Ollama local (opcional):

```bash
LLM_LIVE=1 go test ./internal/ai -run OllamaLive -v
```

Qué se prueba:

- **`analytics/dataset_test.go`**: sobre los CSV reales, exactamente 4 hallazgos, con el tipo, la severidad y el **orden de prioridad** esperados, y la evidencia clave (inicio a las 14:00 del 12-sep, `UNKNOWN` no explica, duración de 12 h, etc.).
- **`analytics/engine_test.go`**: cada detector y regla con series sintéticas, incluido que el ruido normal no genere falsos positivos.
- **`ai`**: plantillas, cliente Ollama y OpenAI con servidores falsos, rechazo de porcentajes inventados y respaldo a plantillas.
- **`service`** y **`httpapi`**: el flujo de la demo de punta a punta sobre HTTP, validaciones, 401, 404, 409 y CORS.
- **`store`**: migraciones idempotentes, seed, JSONB y transacciones contra PostgreSQL real.

| Paquete | Cobertura |
|---|---|
| `analytics` (motor) | 93% |
| `httpapi` | 97% |
| `service` | 91% |
| `config` | 92% |
| `ai` | 89% |
| `ingest` | 88% |
| `store` (con PostgreSQL) | 77% |
| `format` | 100% |

### Frontend

```bash
cd frontend
npm run lint
npm test
npm run build
```

Cubren el login (éxito, error y sesión expirada), el dashboard con y sin análisis, los filtros y el orden de medidores, Run AI Analysis (pasos, resumen y conflicto 409), la investigación con la acción del operador, y el formato de números y fechas.

### Integración continua

`.github/workflows/ci.yml` ejecuta en cada push: `gofmt`, `go vet`, `go test -race` con PostgreSQL, lint, tests y build del frontend, y el build de las imágenes Docker.

---

## 9. Arquitectura

```mermaid
flowchart LR
  UI[React SPA] -- JSON + JWT --> API[API Go · chi]
  API --> SVC[service]
  SVC --> ENG[analytics<br/>motor puro]
  SVC --> EXP[ai · Explainer]
  SVC --> ST[(PostgreSQL)]
  EXP -- opcional --> LLM[Ollama / API compatible con OpenAI]
  EXP -. respaldo .-> TPL[Plantillas]
```

- **`analytics` es puro**: recibe lecturas y eventos y devuelve hallazgos, sin base de datos ni HTTP. Cada regla se prueba de forma aislada.
- Capas `handler → service → repository`. Los servicios dependen de una interfaz (`service.Repository`), implementada por PostgreSQL y por un store en memoria para los tests.
- El análisis corre **asíncrono por pasos** (una goroutine, un análisis a la vez) y persiste su progreso. La UI lo consulta cada 600 ms.

```
backend/
  cmd/api/                 arranque: config → migraciones → seed → HTTP
  internal/analytics/      motor: baseline, detectores, eventos, clasificación, confianza, prioridad
  internal/ai/             explicaciones: plantillas, LLM y guardrails
  internal/service/        casos de uso, auth JWT y runner asíncrono del análisis
  internal/store/          PostgreSQL (pgx) + migraciones embebidas
  internal/httpapi/        router, handlers, middlewares y mapeo de errores
  internal/memstore/       repositorio en memoria para tests
  internal/ingest/         lectura de CSV
  data/                    readings.csv, events.csv, meters.json
frontend/src/
  pages/                   Login, Dashboard, Medidores, Detalle, Anomalías, Investigación
  analysis/                Run AI Analysis + modal de progreso
  components/              UI, badges y gráficos (con vista de tabla accesible)
  api/                     cliente HTTP, tipos y hooks de TanStack Query
```

Documentación ampliada:

- [`docs/architecture.md`](docs/architecture.md): capas, secuencia del análisis, modelo de datos y seguridad.
- [`docs/anomaly-engine.md`](docs/anomaly-engine.md): baseline, detectores, umbrales y su justificación, clasificación, severidad, fórmulas de confianza y prioridad.

---

## 10. API

Base `/api/v1`. Todas las rutas salvo `/auth/login` y `/health` requieren `Authorization: Bearer <token>`. Los errores siguen el formato `{"error": {"code", "message"}}`.

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/auth/login` | `{email, password}` → `{token, expires_at, user}` |
| GET | `/auth/me` | Usuario actual |
| GET | `/dashboard/summary` | KPIs, prioridades, consumo diario y último análisis |
| GET | `/meters` | `status=all\|ok\|alert\|critical`, `q`, `sort=meter\|consumption\|variation\|severity`, `order=asc\|desc` |
| GET | `/meters/:meterId` | Detalle, estadísticas, anomalías y eventos |
| GET | `/meters/:meterId/readings` | Serie con banda esperada. `from`, `to` (RFC3339 o `YYYY-MM-DD HH:MM`), `resolution=hour\|day` |
| GET | `/anomalies` | Hallazgos del último análisis por prioridad. `type`, `severity`, `status`, `meter_id`, `analysis_id` |
| GET | `/anomalies/:id` | Investigación: evidencia, eventos, ranking |
| PATCH | `/anomalies/:id` | Acción del operador: `{status: OPEN\|INVESTIGATING\|RESOLVED\|DISMISSED, note}` |
| POST | `/ai/analyze` | Lanza el análisis → `202` (`409` si ya hay uno en curso) |
| GET | `/ai/analysis/:id` | Progreso por pasos y resumen |
| GET | `/ai/analysis/latest` | Último análisis |
| GET | `/ai/status` | Proveedor de explicaciones y disponibilidad |
| GET | `/health` | Estado del servicio y de la base de datos |

Ejemplo rápido con `curl`:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@energyai.local","password":"demo1234"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')

curl -s -X POST http://localhost:8080/api/v1/ai/analyze -H "Authorization: Bearer $TOKEN"
curl -s "http://localhost:8080/api/v1/anomalies" -H "Authorization: Bearer $TOKEN"
```

Ejemplo de hallazgo:

```json
{
  "meter_id": "M-109",
  "anomaly": true,
  "type": "REAL_ANOMALY",
  "severity": "HIGH",
  "confidence": 0.98,
  "priority_score": 97.4,
  "reason": "Consumo +110,4% frente al baseline desde el 12-sep 14:00 (58 h, sigue activo) sin ningún evento operativo que lo explique...",
  "recommended_action": "Priorizar una inspección en sitio del medidor y la instalación en menos de 24 h...",
  "explanation_source": "LLM:qwen2.5:7b",
  "evidence": { "deviation_pct": 110.4, "variables": [], "signals": [], "events": [], "confidence": {}, "priority": {} }
}
```

---

## 11. Solución de problemas

| Síntoma | Causa y solución |
|---|---|
| `port is already allocated` / `address already in use` (5432, 8080, 8081, 5173) | Otro proceso usa el puerto (por ejemplo, un PostgreSQL local en 5432). Detenlo o cambia el puerto en `docker-compose.yml`, `PORT` o `DATABASE_URL` |
| La API repite `waiting for database` | PostgreSQL no está levantado o `DATABASE_URL` es incorrecta. Revisa `docker compose ps` o el servicio local de PostgreSQL |
| `go: command not found` / `npm: command not found` | Reabre la terminal después de instalar, o agrega Go y Node al `PATH` |
| La barra lateral dice *Ollama no disponible* | Ollama no está corriendo o falta el modelo (`ollama pull qwen2.5:7b`). La app sigue funcionando con explicaciones por reglas |
| En Docker, Ollama no responde | Ollama debe estar corriendo en el host. El contenedor usa `host.docker.internal:11434` (`LLM_BASE_URL` en el `.env` de la raíz) |
| Tras reiniciar el backend la app vuelve al login | El token expiró o cambió `JWT_SECRET`. Inicia sesión de nuevo |
| El frontend muestra "No se pudo conectar con el servidor" | El backend no está en `:8080` (Opción B) o el contenedor `api` está caído (`docker compose logs api`) |
| Quiero volver a los datos originales y borrar los análisis | Docker: `docker compose down -v && docker compose up -d`. Local: `DROP DATABASE energyai; CREATE DATABASE energyai OWNER energy;` y reinicia el backend |
| Los tests del store se saltan | Define `TEST_DATABASE_URL` apuntando a una base cuyo nombre contenga `test` (sección 8) |

---

## 12. Decisiones y límites conocidos

- **Baseline fijo de 7 días.** Todos los eventos del dataset ocurren después. En producción sería una ventana móvil que excluya los periodos anómalos (`BASELINE_DAYS` es configurable).
- **El "ahora" es la última lectura** (14-sep 23:00), no el reloj del sistema. Las fechas se muestran en UTC, igual que en los datos.
- **Un análisis a la vez.** Las decisiones del operador (en investigación, resuelta, descartada) se conservan entre análisis para el mismo medidor y tipo.
- **Umbrales** en `analytics.DefaultConfig()`, justificados en `docs/anomaly-engine.md` (por ejemplo, ±5% es la tolerancia típica de tensión de suministro).
- **Autenticación de demo:** un usuario definido por variables de entorno, JWT HS256 y bcrypt. Cambia `JWT_SECRET` y `DEMO_PASSWORD` fuera de la demo.
