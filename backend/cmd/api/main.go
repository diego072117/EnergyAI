package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"energyai/internal/ai"
	"energyai/internal/analytics"
	"energyai/internal/config"
	"energyai/internal/domain"
	"energyai/internal/httpapi"
	"energyai/internal/ingest"
	"energyai/internal/service"
	"energyai/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(".env")
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Connect(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := seed(ctx, st, cfg.DataDir, log); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	if n, err := st.FailInterruptedRuns(ctx); err != nil {
		return err
	} else if n > 0 {
		log.Warn("marked interrupted analyses as failed", "count", n)
	}

	engine := analytics.DefaultConfig()
	engine.BaselineDays = cfg.BaselineDays

	auth := service.NewAuth(st, cfg.JWTSecret, cfg.JWTTTL)
	if err := auth.EnsureUser(ctx, cfg.DemoEmail, cfg.DemoName, cfg.DemoPassword); err != nil {
		return fmt.Errorf("demo user: %w", err)
	}

	explainer := newExplainer(cfg, log)
	runner := service.NewRunner(st, explainer, engine, service.RunnerOptions{StepDelay: cfg.StepDelay}, log)
	handler := httpapi.NewRouter(httpapi.Deps{
		Auth:        auth,
		Meters:      service.NewMeterService(st, engine),
		Anomalies:   service.NewAnomalyService(st),
		Dashboard:   service.NewDashboardService(st, engine),
		Runner:      runner,
		AIStatus:    explainer,
		Health:      st.Ping,
		CORSOrigins: cfg.CORSOrigins,
		Log:         log,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("API listening", "addr", srv.Addr, "llm_provider", cfg.LLMProvider)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	runner.Shutdown()
	return err
}

type explainer interface {
	ai.Explainer
	ai.StatusReporter
}

func newExplainer(cfg config.Config, log *slog.Logger) explainer {
	if cfg.LLMProvider == "none" {
		return ai.Template{}
	}
	log.Info("generative explanations enabled", "provider", cfg.LLM.Provider, "model", cfg.LLM.Model, "base_url", cfg.LLM.BaseURL)
	return ai.WithFallback{Primary: ai.NewLLM(cfg.LLM), Fallback: ai.Template{}, Log: log}
}

func seed(ctx context.Context, st *store.Store, dir string, log *slog.Logger) error {
	readings, err := ingest.ReadReadingsFile(filepath.Join(dir, "readings.csv"))
	if err != nil {
		return err
	}
	events, err := ingest.ReadEventsFile(filepath.Join(dir, "events.csv"))
	if err != nil {
		return err
	}
	metas, err := meterMetadata(filepath.Join(dir, "meters.json"), readings)
	if err != nil {
		return err
	}
	inserted, err := st.SeedIfEmpty(ctx, metas, readings, events)
	if err != nil {
		return err
	}
	if inserted {
		log.Info("database seeded", "meters", len(metas), "readings", len(readings), "events", len(events))
	}
	return nil
}

func meterMetadata(path string, readings []domain.Reading) ([]store.MeterMeta, error) {
	known := map[string]store.MeterMeta{}
	if b, err := os.ReadFile(path); err == nil {
		var list []store.MeterMeta
		if err := json.Unmarshal(b, &list); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, m := range list {
			known[m.MeterID] = m
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var out []store.MeterMeta
	for _, in := range analytics.BuildInputs(readings, nil) {
		m, ok := known[in.MeterID]
		if !ok {
			m = store.MeterMeta{MeterID: in.MeterID, Name: "Medidor " + in.MeterID, Location: "Sin ubicación"}
		}
		out = append(out, m)
	}
	return out, nil
}
