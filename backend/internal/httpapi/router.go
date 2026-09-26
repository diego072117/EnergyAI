package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"energyai/internal/ai"
	"energyai/internal/service"
)

type Deps struct {
	Auth        *service.Auth
	Meters      *service.MeterService
	Anomalies   *service.AnomalyService
	Dashboard   *service.DashboardService
	Runner      *service.Runner
	AIStatus    ai.StatusReporter
	Health      func(ctx context.Context) error
	CORSOrigins []string
	Log         *slog.Logger
}

type api struct{ Deps }

func NewRouter(d Deps) http.Handler {
	a := &api{d}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, a.requestLogger, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: d.CORSOrigins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/health", a.health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", a.login)
		r.Group(func(r chi.Router) {
			r.Use(a.authenticate)
			r.Get("/auth/me", a.me)
			r.Get("/dashboard/summary", a.dashboard)
			r.Get("/meters", a.listMeters)
			r.Get("/meters/{meterId}", a.getMeter)
			r.Get("/meters/{meterId}/readings", a.meterReadings)
			r.Get("/anomalies", a.listAnomalies)
			r.Get("/anomalies/{id}", a.getAnomaly)
			r.Patch("/anomalies/{id}", a.updateAnomaly)
			r.Post("/ai/analyze", a.analyze)
			r.Get("/ai/analysis/latest", a.latestAnalysis)
			r.Get("/ai/analysis/{id}", a.getAnalysis)
			r.Get("/ai/status", a.aiStatus)
		})
	})
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "ruta no encontrada")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "método no permitido")
	})
	return r
}

type ctxKey struct{}

func (a *api) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "falta el token de acceso")
			return
		}
		claims, err := a.Auth.Verify(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "token inválido o expirado")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
	})
}

func (a *api) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		level := slog.LevelDebug
		if ww.Status() >= 500 {
			level = slog.LevelError
		} else if !strings.HasPrefix(r.URL.Path, "/api/v1/ai/analysis/") { // polling is noisy
			level = slog.LevelInfo
		}
		a.Log.Log(r.Context(), level, "http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(), "request_id", middleware.GetReqID(r.Context()))
	})
}
