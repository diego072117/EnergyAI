package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"energyai/internal/ai"
	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/ingest"
	"energyai/internal/memstore"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func newRepo(t *testing.T) *memstore.Store {
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
	var meters []domain.Meter
	for _, in := range analytics.BuildInputs(rs, nil) {
		meters = append(meters, domain.Meter{MeterID: in.MeterID, Name: "Medidor " + in.MeterID, Location: "Planta"})
	}
	return memstore.New(meters, rs, evs)
}

func runAnalysis(t *testing.T, repo Repository, explainer ai.Explainer) domain.AnalysisRun {
	t.Helper()
	r := NewRunner(repo, explainer, analytics.DefaultConfig(), RunnerOptions{}, quiet)
	run, err := r.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r.Wait()
	got, err := r.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRunnerCompletesAllSteps(t *testing.T) {
	repo := newRepo(t)
	run := runAnalysis(t, repo, ai.Template{})
	if run.Status != domain.RunCompleted {
		t.Fatalf("status %s: %s", run.Status, run.Error)
	}
	for _, s := range run.Steps {
		if s.State != domain.StepDone || s.Detail == "" || s.StartedAt == nil || s.EndedAt == nil {
			t.Errorf("step %s not completed: %+v", s.Name, s)
		}
	}
	if run.Summary == nil || run.Summary.Detected != 4 || run.Summary.HighPriority != 2 {
		t.Fatalf("summary %+v", run.Summary)
	}
	if run.Summary.Text != "4 anomalías detectadas · 2 requieren atención prioritaria" {
		t.Errorf("summary text %q", run.Summary.Text)
	}
	meters, _ := repo.ListMeters(context.Background())
	want := map[string]domain.MeterStatus{"M-109": domain.MeterCritical, "M-112": domain.MeterAlert, "M-104": domain.MeterAlert, "M-106": domain.MeterOK, "M-101": domain.MeterOK}
	for _, m := range meters {
		if w, ok := want[m.MeterID]; ok && m.Status != w {
			t.Errorf("%s status %s, want %s", m.MeterID, m.Status, w)
		}
	}
}

func TestRunnerRejectsConcurrentRuns(t *testing.T) {
	r := NewRunner(newRepo(t), ai.Template{}, analytics.DefaultConfig(), RunnerOptions{StepDelay: 50 * time.Millisecond}, quiet)
	if _, err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Start(context.Background()); !errors.Is(err, ErrAnalysisRunning) {
		t.Fatalf("expected ErrAnalysisRunning, got %v", err)
	}
	r.Wait()
	if _, err := r.Start(context.Background()); err != nil {
		t.Fatalf("a new run must be allowed after completion: %v", err)
	}
	r.Shutdown()
}

type failingExplainer struct{}

func (failingExplainer) Explain(context.Context, analytics.Finding) (ai.Explanation, error) {
	return ai.Explanation{}, errors.New("boom")
}

func TestRunnerMarksFailedRuns(t *testing.T) {
	run := runAnalysis(t, newRepo(t), failingExplainer{})
	if run.Status != domain.RunFailed || !strings.Contains(run.Error, "boom") {
		t.Fatalf("expected failed run, got %s %q", run.Status, run.Error)
	}
	var failed bool
	for _, s := range run.Steps {
		if s.Name == domain.StepExplanation && s.State == domain.StepFailed {
			failed = true
		}
	}
	if !failed {
		t.Error("the explanation step should be marked as failed")
	}
}

func TestRunnerUsesFallbackExplainer(t *testing.T) {
	run := runAnalysis(t, newRepo(t), ai.WithFallback{Primary: failingExplainer{}, Fallback: ai.Template{}})
	if run.Status != domain.RunCompleted || run.Summary.ExplanationSource != ai.SourceTemplate {
		t.Fatalf("got %s / %+v", run.Status, run.Summary)
	}
}

func TestStatusCarriesOverBetweenRuns(t *testing.T) {
	repo := newRepo(t)
	runAnalysis(t, repo, ai.Template{})
	svc := NewAnomalyService(repo)
	list, err := svc.List(context.Background(), AnomalyQuery{})
	if err != nil {
		t.Fatal(err)
	}
	top := list.Items[0]
	note := "Técnico asignado"
	if _, err := svc.UpdateStatus(context.Background(), top.ID, "investigating", &note); err != nil {
		t.Fatal(err)
	}
	runAnalysis(t, repo, ai.Template{})
	list, _ = svc.List(context.Background(), AnomalyQuery{})
	if list.Items[0].MeterID != top.MeterID || list.Items[0].Status != domain.StatusInvestigating || list.Items[0].Note != note {
		t.Fatalf("status not carried over: %+v", list.Items[0])
	}
}

func TestAnomalyServiceFiltersAndDetail(t *testing.T) {
	repo := newRepo(t)
	svc := NewAnomalyService(repo)
	empty, err := svc.List(context.Background(), AnomalyQuery{})
	if err != nil || empty.Total != 0 || empty.Items == nil {
		t.Fatalf("before any run: %+v %v", empty, err)
	}
	run := runAnalysis(t, repo, ai.Template{})

	high, err := svc.List(context.Background(), AnomalyQuery{Severity: "high"})
	if err != nil || high.Total != 2 || high.AnalysisID != run.ID {
		t.Fatalf("high: %+v %v", high, err)
	}
	dq, _ := svc.List(context.Background(), AnomalyQuery{Type: "DATA_QUALITY"})
	if dq.Total != 1 || dq.Items[0].MeterID != "M-112" {
		t.Fatalf("dq: %+v", dq)
	}
	for _, q := range []AnomalyQuery{{Type: "x"}, {Severity: "x"}, {Status: "x"}} {
		if _, err := svc.List(context.Background(), q); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: expected invalid input, got %v", q, err)
		}
	}
	if _, err := svc.List(context.Background(), AnomalyQuery{AnalysisID: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown run: %v", err)
	}

	d, err := svc.Get(context.Background(), high.Items[0].ID)
	if err != nil || d.Rank != 1 || d.Of != 4 || d.Meter.MeterID != "M-109" {
		t.Fatalf("detail: rank %d of %d meter %s err %v", d.Rank, d.Of, d.Meter.MeterID, err)
	}
	if _, err := svc.Get(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected not found, got %v", err)
	}
	if _, err := svc.UpdateStatus(context.Background(), d.ID, "closed", nil); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected invalid status, got %v", err)
	}
}

func TestResolvingClearsMeterStatus(t *testing.T) {
	repo := newRepo(t)
	runAnalysis(t, repo, ai.Template{})
	svc := NewAnomalyService(repo)
	list, _ := svc.List(context.Background(), AnomalyQuery{MeterID: "M-109"})
	if _, err := svc.UpdateStatus(context.Background(), list.Items[0].ID, "RESOLVED", nil); err != nil {
		t.Fatal(err)
	}
	m, _ := repo.GetMeter(context.Background(), "M-109")
	if m.Status != domain.MeterOK {
		t.Errorf("resolved meter should be OK, got %s", m.Status)
	}
}

func TestMeterList(t *testing.T) {
	repo := newRepo(t)
	svc := NewMeterService(repo, analytics.DefaultConfig())
	runAnalysis(t, repo, ai.Template{})

	all, err := svc.List(context.Background(), MeterFilter{})
	if err != nil || len(all) != 12 {
		t.Fatalf("all: %d %v", len(all), err)
	}
	if all[0].MeterID != "M-109" || all[0].Status != domain.MeterCritical || all[0].Anomaly == nil {
		t.Errorf("default sort must put the most severe first, got %s", all[0].MeterID)
	}
	alerts, _ := svc.List(context.Background(), MeterFilter{Status: "alert"})
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(alerts))
	}
	normal, _ := svc.List(context.Background(), MeterFilter{Status: "ok"})
	if len(normal) != 9 {
		t.Errorf("expected 9 normal meters, got %d", len(normal))
	}
	search, _ := svc.List(context.Background(), MeterFilter{Query: "m-10"})
	if len(search) != 9 {
		t.Errorf("search m-10 should match M-101..M-109, got %d", len(search))
	}
	byVar, _ := svc.List(context.Background(), MeterFilter{Sort: "variation", Order: "desc"})
	if byVar[0].MeterID != "M-109" || byVar[1].MeterID != "M-104" {
		t.Errorf("variation sort: %s, %s", byVar[0].MeterID, byVar[1].MeterID)
	}
	byCons, _ := svc.List(context.Background(), MeterFilter{Sort: "consumption", Order: "asc"})
	if byCons[0].Last24hKWh > byCons[len(byCons)-1].Last24hKWh {
		t.Error("consumption asc not ordered")
	}
	byID, _ := svc.List(context.Background(), MeterFilter{Sort: "meter"})
	if byID[0].MeterID != "M-101" {
		t.Errorf("meter sort: %s", byID[0].MeterID)
	}
	for _, f := range []MeterFilter{{Status: "x"}, {Sort: "x"}, {Order: "x"}} {
		if _, err := svc.List(context.Background(), f); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: expected invalid input", f)
		}
	}
}

func TestMeterDetailAndReadings(t *testing.T) {
	repo := newRepo(t)
	svc := NewMeterService(repo, analytics.DefaultConfig())
	runAnalysis(t, repo, ai.Template{})

	d, err := svc.Get(context.Background(), "M-109")
	if err != nil {
		t.Fatal(err)
	}
	if d.VariationPct < 100 || d.Baseline == nil || len(d.Anomalies) != 1 || len(d.Events) != 1 {
		t.Errorf("detail %+v", d.MeterSummary)
	}
	if _, err := svc.Get(context.Background(), "M-999"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected not found, got %v", err)
	}

	hourly, err := svc.Readings(context.Background(), "M-109", ReadingsParams{})
	if err != nil || len(hourly.Points) != 336 || hourly.Points[0].ExpectedKWh == 0 {
		t.Fatalf("hourly: %d points, %v", len(hourly.Points), err)
	}
	from := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	daily, err := svc.Readings(context.Background(), "M-109", ReadingsParams{From: &from, Resolution: "day"})
	if err != nil || len(daily.Points) != 3 {
		t.Fatalf("daily: %d points, %v", len(daily.Points), err)
	}
	last := daily.Points[2]
	if last.ConsumptionKWh < 2000 || last.ExpectedKWh < 1000 || last.UpperKWh <= last.ExpectedKWh {
		t.Errorf("daily point %+v", last)
	}
	to := from.Add(-time.Hour)
	if _, err := svc.Readings(context.Background(), "M-109", ReadingsParams{From: &from, To: &to}); !errors.Is(err, ErrInvalidInput) {
		t.Error("expected invalid range")
	}
	if _, err := svc.Readings(context.Background(), "M-109", ReadingsParams{Resolution: "week"}); !errors.Is(err, ErrInvalidInput) {
		t.Error("expected invalid resolution")
	}
	if _, err := svc.Readings(context.Background(), "M-999", ReadingsParams{}); !errors.Is(err, ErrNotFound) {
		t.Error("expected not found")
	}
}

func TestDashboardSummary(t *testing.T) {
	repo := newRepo(t)
	svc := NewDashboardService(repo, analytics.DefaultConfig())
	before, err := svc.Summary(context.Background())
	if err != nil || before.LastAnalysis != nil || before.Anomalies.Detected != 0 {
		t.Fatalf("before analysis: %+v %v", before, err)
	}
	runAnalysis(t, repo, ai.Template{})
	s, err := svc.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Meters != 12 || s.Anomalies.Detected != 4 || s.Anomalies.HighPriority != 2 || len(s.TopPriority) != 2 {
		t.Errorf("summary %+v", s.Anomalies)
	}
	if s.TopPriority[0].MeterID != "M-109" || s.MetersByStatus["CRITICAL"] != 1 {
		t.Errorf("top priority %s, statuses %v", s.TopPriority[0].MeterID, s.MetersByStatus)
	}
	if len(s.DailyConsumption) != 14 || s.TotalConsumptionKWh < 155000 || s.TotalConsumptionKWh > 156000 {
		t.Errorf("consumption %v over %d days", s.TotalConsumptionKWh, len(s.DailyConsumption))
	}
	if s.AvgConfidence < 0.85 {
		t.Errorf("avg confidence %v", s.AvgConfidence)
	}
}

func TestAuth(t *testing.T) {
	repo := newRepo(t)
	a := NewAuth(repo, "secret", time.Hour)
	if err := a.EnsureUser(context.Background(), " Demo@EnergyAI.local ", "Demo", "pass1234"); err != nil {
		t.Fatal(err)
	}
	res, err := a.Login(context.Background(), "demo@energyai.local", "pass1234")
	if err != nil || res.Token == "" || res.User.Name != "Demo" {
		t.Fatalf("login: %+v %v", res, err)
	}
	claims, err := a.Verify(res.Token)
	if err != nil || claims.Email != "demo@energyai.local" {
		t.Fatalf("verify: %+v %v", claims, err)
	}
	if _, err := a.Login(context.Background(), "demo@energyai.local", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password: %v", err)
	}
	if _, err := a.Login(context.Background(), "nobody@x.io", "x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user: %v", err)
	}
	if _, err := NewAuth(repo, "other-secret", time.Hour).Verify(res.Token); err == nil {
		t.Error("token signed with another secret must be rejected")
	}
	expired := NewAuth(repo, "secret", time.Hour)
	expired.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, err := expired.Verify(res.Token); err == nil {
		t.Error("expired token must be rejected")
	}
}
