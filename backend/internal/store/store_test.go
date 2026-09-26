package store_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"energyai/internal/domain"
	"energyai/internal/ingest"
	"energyai/internal/store"
)

// These tests need a real PostgreSQL. Run them with:
//
//	TEST_DATABASE_URL=postgres://energy:energy@localhost:5432/energyai_test?sslmode=disable go test ./internal/store/
func openStore(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	// These tests truncate every table: refuse anything that is not a test database.
	if !strings.Contains(url, "test") {
		t.Fatalf("TEST_DATABASE_URL must point to a dedicated test database (its name must contain \"test\")")
	}
	ctx := context.Background()
	st, err := store.Connect(ctx, url, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrations must be idempotent: %v", err)
	}
	if err := st.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	return st
}

func seed(t *testing.T, st *store.Store) {
	t.Helper()
	dir := filepath.Join("..", "..", "data")
	rs, err := ingest.ReadReadingsFile(filepath.Join(dir, "readings.csv"))
	if err != nil {
		t.Fatal(err)
	}
	evs, err := ingest.ReadEventsFile(filepath.Join(dir, "events.csv"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var metas []store.MeterMeta
	for _, r := range rs {
		if !seen[r.MeterID] {
			seen[r.MeterID] = true
			metas = append(metas, store.MeterMeta{MeterID: r.MeterID, Name: "Medidor " + r.MeterID, Location: "Planta"})
		}
	}
	ok, err := st.SeedIfEmpty(context.Background(), metas, rs, evs)
	if err != nil || !ok {
		t.Fatalf("seed: %v %v", ok, err)
	}
	again, err := st.SeedIfEmpty(context.Background(), metas, rs, evs)
	if err != nil || again {
		t.Fatalf("seed must be idempotent: %v %v", again, err)
	}
}

func TestSeedAndQueries(t *testing.T) {
	st := openStore(t)
	seed(t, st)
	ctx := context.Background()

	meters, err := st.ListMeters(ctx)
	if err != nil || len(meters) != 12 || meters[0].MeterID != "M-101" || meters[0].Status != domain.MeterOK {
		t.Fatalf("meters %d %v", len(meters), err)
	}
	if _, err := st.GetMeter(ctx, "M-999"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	all, _ := st.ListReadings(ctx, store.ReadingsQuery{})
	if len(all) != 4032 {
		t.Errorf("readings %d", len(all))
	}
	from := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	to := from.Add(5 * time.Hour)
	window, _ := st.ListReadings(ctx, store.ReadingsQuery{MeterID: "M-109", From: &from, To: &to})
	if len(window) != 6 || !window[0].Timestamp.Equal(from) || window[0].Timestamp.Location() != time.UTC {
		t.Errorf("window %d readings", len(window))
	}
	evs, _ := st.ListEvents(ctx, "M-109")
	if len(evs) != 1 || evs[0].Type != domain.EventUnknown {
		t.Errorf("events %+v", evs)
	}
}

func sampleAnomaly(meter string, typ domain.AnomalyType, sev domain.Severity, prio float64) domain.Anomaly {
	ts := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	return domain.Anomaly{
		MeterID: meter, DetectedAt: ts, Type: typ, IsAnomaly: typ != domain.AnomalyFalsePositive, Severity: sev,
		Confidence: 0.9, PriorityScore: prio, Reason: "r", RecommendedAction: "a", EvidenceSummary: []string{"e1"},
		WindowStart: ts, WindowEnd: ts.Add(10 * time.Hour), ExplanationSource: "TEMPLATE",
		Evidence: domain.Evidence{DeviationPct: 110.4, Signals: []domain.Signal{{Code: "LEVEL_SHIFT", Supports: true}}},
	}
}

func completeRun(t *testing.T, st *store.Store, as ...domain.Anomaly) (domain.AnalysisRun, []domain.Anomaly) {
	t.Helper()
	ctx := context.Background()
	run := domain.AnalysisRun{ID: time.Now().Format("150405.000000000"), Status: domain.RunPending, Steps: domain.NewAnalysisSteps(), StartedAt: time.Now().UTC()}
	if err := st.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Status, run.CurrentStep = domain.RunRunning, domain.StepBaseline
	if err := st.UpdateRunProgress(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Summary = &domain.AnalysisSummary{Detected: len(as), ByType: map[string]int{}}
	saved, err := st.CompleteRun(ctx, run, as)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.GetRun(ctx, run.ID)
	if err != nil || got.Status != domain.RunCompleted || got.Summary == nil || got.FinishedAt == nil {
		t.Fatalf("run %+v %v", got, err)
	}
	return got, saved
}

func TestRunLifecycleAndAnomalies(t *testing.T) {
	st := openStore(t)
	seed(t, st)
	ctx := context.Background()
	if _, err := st.LatestRun(ctx, true); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected no runs, got %v", err)
	}

	run, saved := completeRun(t, st,
		sampleAnomaly("M-109", domain.AnomalyReal, domain.SeverityHigh, 97),
		sampleAnomaly("M-106", domain.AnomalyFalsePositive, domain.SeverityLow, 15),
		sampleAnomaly("M-112", domain.AnomalyDataQuality, domain.SeverityHigh, 86),
	)
	list, err := st.ListAnomalies(ctx, store.AnomalyFilter{AnalysisID: run.ID})
	if err != nil || len(list) != 3 || list[0].MeterID != "M-109" || list[2].MeterID != "M-106" {
		t.Fatalf("list %v %v", list, err)
	}
	if list[0].Evidence.DeviationPct != 110.4 || len(list[0].EvidenceSummary) != 1 {
		t.Errorf("JSONB round trip failed: %+v", list[0].Evidence)
	}
	high, _ := st.ListAnomalies(ctx, store.AnomalyFilter{AnalysisID: run.ID, Severity: domain.SeverityHigh, Type: domain.AnomalyDataQuality})
	if len(high) != 1 {
		t.Errorf("filtered %d", len(high))
	}
	m109, _ := st.GetMeter(ctx, "M-109")
	m112, _ := st.GetMeter(ctx, "M-112")
	m106, _ := st.GetMeter(ctx, "M-106")
	if m109.Status != domain.MeterCritical || m112.Status != domain.MeterAlert || m106.Status != domain.MeterOK {
		t.Errorf("statuses %s %s %s", m109.Status, m112.Status, m106.Status)
	}

	note := "revisado"
	upd, err := st.UpdateAnomalyStatus(ctx, saved[0].ID, domain.StatusResolved, &note)
	if err != nil || upd.Status != domain.StatusResolved || upd.Note != note {
		t.Fatalf("update %+v %v", upd, err)
	}
	if m, _ := st.GetMeter(ctx, "M-109"); m.Status != domain.MeterOK {
		t.Errorf("resolved meter should be OK, got %s", m.Status)
	}
	if _, err := st.UpdateAnomalyStatus(ctx, 99999, domain.StatusResolved, nil); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// A new run keeps the operator decision for the same meter and type.
	run2, _ := completeRun(t, st, sampleAnomaly("M-109", domain.AnomalyReal, domain.SeverityHigh, 97))
	again, _ := st.ListAnomalies(ctx, store.AnomalyFilter{AnalysisID: run2.ID})
	if again[0].Status != domain.StatusResolved || again[0].Note != note {
		t.Errorf("status not carried over: %+v", again[0])
	}
	if latest, _ := st.LatestRun(ctx, true); latest.ID != run2.ID {
		t.Errorf("latest %s", latest.ID)
	}
	if m, _ := st.GetMeter(ctx, "M-112"); m.Status != domain.MeterOK {
		t.Errorf("meters without findings in the new run must be OK, got %s", m.Status)
	}
}

func TestFailedAndInterruptedRuns(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	run := domain.AnalysisRun{ID: "r-fail", Status: domain.RunRunning, Steps: domain.NewAnalysisSteps(), StartedAt: time.Now().UTC()}
	if err := st.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Error = "boom"
	if err := st.FailRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetRun(ctx, "r-fail")
	if got.Status != domain.RunFailed || got.Error != "boom" {
		t.Errorf("failed run %+v", got)
	}
	stuck := domain.AnalysisRun{ID: "r-stuck", Status: domain.RunRunning, Steps: domain.NewAnalysisSteps(), StartedAt: time.Now().UTC()}
	_ = st.CreateRun(ctx, stuck)
	if n, err := st.FailInterruptedRuns(ctx); err != nil || n != 1 {
		t.Errorf("interrupted %d %v", n, err)
	}
	if _, err := st.GetRun(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUsers(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	if err := st.UpsertUser(ctx, "a@b.c", "A", "hash1"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertUser(ctx, "a@b.c", "A2", "hash2"); err != nil {
		t.Fatal(err)
	}
	u, err := st.GetUserByEmail(ctx, "a@b.c")
	if err != nil || u.Name != "A2" || u.PasswordHash != "hash2" {
		t.Fatalf("user %+v %v", u, err)
	}
	if _, err := st.GetUserByEmail(ctx, "x@y.z"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
