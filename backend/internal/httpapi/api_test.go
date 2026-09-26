package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"energyai/internal/ai"
	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/ingest"
	"energyai/internal/memstore"
	"energyai/internal/service"
)

type testAPI struct {
	t      *testing.T
	srv    *httptest.Server
	token  string
	runner *service.Runner
}

func newTestAPI(t *testing.T, health func(context.Context) error, stepDelay ...time.Duration) *testAPI {
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
		meters = append(meters, domain.Meter{MeterID: in.MeterID, Name: "Medidor " + in.MeterID})
	}
	repo := memstore.New(meters, rs, evs)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := analytics.DefaultConfig()
	auth := service.NewAuth(repo, "test-secret", time.Hour)
	if err := auth.EnsureUser(context.Background(), "demo@energyai.local", "Demo", "demo1234"); err != nil {
		t.Fatal(err)
	}
	opts := service.RunnerOptions{}
	if len(stepDelay) > 0 {
		opts.StepDelay = stepDelay[0]
	}
	runner := service.NewRunner(repo, ai.Template{}, cfg, opts, log)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth, Meters: service.NewMeterService(repo, cfg), Anomalies: service.NewAnomalyService(repo),
		Dashboard: service.NewDashboardService(repo, cfg), Runner: runner, AIStatus: ai.Template{},
		Health: health, CORSOrigins: []string{"http://localhost:5173"}, Log: log,
	}))
	t.Cleanup(func() { runner.Shutdown(); srv.Close() })
	return &testAPI{t: t, srv: srv, runner: runner}
}

func (a *testAPI) do(method, path string, body any, out any) int {
	a.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, a.srv.URL+path, rd)
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			a.t.Fatalf("%s %s: decoding: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func (a *testAPI) login() {
	a.t.Helper()
	var res service.LoginResult
	if code := a.do("POST", "/api/v1/auth/login", map[string]string{"email": "demo@energyai.local", "password": "demo1234"}, &res); code != 200 {
		a.t.Fatalf("login status %d", code)
	}
	a.token = res.Token
}

func TestHealth(t *testing.T) {
	a := newTestAPI(t, func(context.Context) error { return nil })
	var body map[string]string
	if code := a.do("GET", "/health", nil, &body); code != 200 || body["status"] != "ok" {
		t.Fatalf("health %d %v", code, body)
	}
	down := newTestAPI(t, func(context.Context) error { return errors.New("down") })
	if code := down.do("GET", "/health", nil, nil); code != http.StatusServiceUnavailable {
		t.Errorf("degraded health should return 503, got %d", code)
	}
}

func TestAuthRequired(t *testing.T) {
	a := newTestAPI(t, nil)
	var e map[string]APIError
	if code := a.do("GET", "/api/v1/meters", nil, &e); code != 401 || e["error"].Code != "unauthorized" {
		t.Fatalf("expected 401, got %d %v", code, e)
	}
	a.token = "garbage"
	if code := a.do("GET", "/api/v1/meters", nil, nil); code != 401 {
		t.Fatalf("invalid token: expected 401, got %d", code)
	}
}

func TestLoginErrors(t *testing.T) {
	a := newTestAPI(t, nil)
	cases := []struct {
		body any
		code int
	}{
		{map[string]string{"email": "demo@energyai.local", "password": "bad"}, 401},
		{map[string]string{"email": "", "password": ""}, 400},
		{map[string]any{"email": "x", "password": "y", "extra": 1}, 400},
	}
	for _, c := range cases {
		if code := a.do("POST", "/api/v1/auth/login", c.body, nil); code != c.code {
			t.Errorf("%v: expected %d, got %d", c.body, c.code, code)
		}
	}
}

// TestDemoFlow walks the demo script end to end:
// login → dashboard → run analysis → poll → anomalies → investigation → action.
func TestDemoFlow(t *testing.T) {
	a := newTestAPI(t, nil)
	a.login()

	var me map[string]string
	if a.do("GET", "/api/v1/auth/me", nil, &me); me["email"] != "demo@energyai.local" {
		t.Errorf("me = %v", me)
	}

	var before service.DashboardSummary
	a.do("GET", "/api/v1/dashboard/summary", nil, &before)
	if before.LastAnalysis != nil || before.Meters != 12 {
		t.Fatalf("dashboard before analysis %+v", before)
	}
	if code := a.do("GET", "/api/v1/ai/analysis/latest", nil, nil); code != 404 {
		t.Errorf("latest before any run: %d", code)
	}

	var run domain.AnalysisRun
	if code := a.do("POST", "/api/v1/ai/analyze", nil, &run); code != http.StatusAccepted || run.ID == "" {
		t.Fatalf("analyze %d %+v", code, run)
	}
	a.runner.Wait()
	var done domain.AnalysisRun
	a.do("GET", "/api/v1/ai/analysis/"+run.ID, nil, &done)
	if done.Status != domain.RunCompleted || done.Summary.Detected != 4 {
		t.Fatalf("run %+v", done)
	}
	var latest domain.AnalysisRun
	if a.do("GET", "/api/v1/ai/analysis/latest", nil, &latest); latest.ID != run.ID {
		t.Errorf("latest %s", latest.ID)
	}

	var dash service.DashboardSummary
	a.do("GET", "/api/v1/dashboard/summary", nil, &dash)
	if dash.Anomalies.Detected != 4 || dash.Anomalies.HighPriority != 2 || dash.TopPriority[0].MeterID != "M-109" {
		t.Fatalf("dashboard %+v", dash.Anomalies)
	}

	var meters struct {
		Items []service.MeterSummary `json:"items"`
		Total int                    `json:"total"`
	}
	a.do("GET", "/api/v1/meters?status=critical", nil, &meters)
	if meters.Total != 1 || meters.Items[0].MeterID != "M-109" {
		t.Fatalf("critical meters %+v", meters)
	}

	var detail service.MeterDetail
	a.do("GET", "/api/v1/meters/M-109", nil, &detail)
	if detail.Status != domain.MeterCritical || len(detail.Anomalies) != 1 {
		t.Fatalf("detail %+v", detail.MeterSummary)
	}
	var series service.ReadingSeries
	a.do("GET", "/api/v1/meters/M-109/readings?from=2026-09-12T00:00:00Z&resolution=day", nil, &series)
	if len(series.Points) != 3 || series.Baseline == nil {
		t.Fatalf("series %d points", len(series.Points))
	}

	var list service.AnomalyList
	a.do("GET", "/api/v1/anomalies", nil, &list)
	if list.Total != 4 || list.Items[0].MeterID != "M-109" || list.Items[3].Type != domain.AnomalyFalsePositive {
		t.Fatalf("anomalies %+v", list)
	}
	top := list.Items[0]
	var inv service.AnomalyDetail
	a.do("GET", "/api/v1/anomalies/"+itoa(top.ID), nil, &inv)
	if inv.Rank != 1 || inv.Reason == "" || inv.RecommendedAction == "" || len(inv.Evidence.Signals) == 0 {
		t.Fatalf("investigation %+v", inv)
	}

	var updated domain.Anomaly
	code := a.do("PATCH", "/api/v1/anomalies/"+itoa(top.ID), map[string]string{"status": "INVESTIGATING", "note": "Cuadrilla enviada"}, &updated)
	if code != 200 || updated.Status != domain.StatusInvestigating || updated.Note != "Cuadrilla enviada" {
		t.Fatalf("update %d %+v", code, updated)
	}

	var st ai.Status
	if a.do("GET", "/api/v1/ai/status", nil, &st); !st.Available || st.Provider != "template" {
		t.Errorf("ai status %+v", st)
	}
}

func TestValidationErrors(t *testing.T) {
	a := newTestAPI(t, nil)
	a.login()
	cases := []struct {
		method, path string
		body         any
		code         int
	}{
		{"GET", "/api/v1/meters?status=weird", nil, 400},
		{"GET", "/api/v1/meters/M-999", nil, 404},
		{"GET", "/api/v1/meters/M-109/readings?from=ayer", nil, 400},
		{"GET", "/api/v1/meters/M-109/readings?to=bad", nil, 400},
		{"GET", "/api/v1/meters/M-109/readings?resolution=week", nil, 400},
		{"GET", "/api/v1/anomalies?severity=urgent", nil, 400},
		{"GET", "/api/v1/anomalies/abc", nil, 400},
		{"GET", "/api/v1/anomalies/999", nil, 404},
		{"PATCH", "/api/v1/anomalies/abc", map[string]string{"status": "OPEN"}, 400},
		{"PATCH", "/api/v1/anomalies/1", map[string]string{"status": "DONE"}, 400},
		{"PATCH", "/api/v1/anomalies/1", map[string]int{"status": 1}, 400},
		{"GET", "/api/v1/ai/analysis/nope", nil, 404},
		{"GET", "/api/v1/nothing", nil, 404},
		{"DELETE", "/api/v1/meters", nil, 405},
	}
	for _, c := range cases {
		var e map[string]APIError
		if code := a.do(c.method, c.path, c.body, &e); code != c.code || e["error"].Message == "" {
			t.Errorf("%s %s: expected %d, got %d %v", c.method, c.path, c.code, code, e)
		}
	}
}

func TestConcurrentAnalysisReturnsConflict(t *testing.T) {
	// Each step lasts at least 100ms, so the first run is still active when
	// the second request arrives.
	a := newTestAPI(t, nil, 100*time.Millisecond)
	a.login()
	if code := a.do("POST", "/api/v1/ai/analyze", nil, nil); code != http.StatusAccepted {
		t.Fatalf("first run: %d", code)
	}
	var e map[string]APIError
	if code := a.do("POST", "/api/v1/ai/analyze", nil, &e); code != http.StatusConflict || e["error"].Code != "analysis_running" {
		t.Fatalf("second run: %d %v", code, e)
	}
	a.runner.Wait()
}

func TestCORSPreflight(t *testing.T) {
	a := newTestAPI(t, nil)
	req, _ := http.NewRequest("OPTIONS", a.srv.URL+"/api/v1/meters", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allow origin %q", got)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return strings.TrimSpace(string(b))
}
