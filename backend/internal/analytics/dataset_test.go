package analytics_test

import (
	"path/filepath"
	"testing"

	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/ingest"
)

// loadDataset runs the engine over the real CSV files shipped with the repo.
// Meter IDs appear only in this test, never in the engine logic.
func loadDataset(t *testing.T) analytics.Result {
	t.Helper()
	dir := filepath.Join("..", "..", "data")
	readings, err := ingest.ReadReadingsFile(filepath.Join(dir, "readings.csv"))
	if err != nil {
		t.Fatal(err)
	}
	events, err := ingest.ReadEventsFile(filepath.Join(dir, "events.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 4032 {
		t.Fatalf("expected 4032 readings, got %d", len(readings))
	}
	return analytics.Run(analytics.DefaultConfig(), analytics.BuildInputs(readings, events))
}

func TestDataset_ExpectedFindings(t *testing.T) {
	res := loadDataset(t)
	for _, f := range res.Findings {
		t.Logf("%s %-20s %-6s conf=%.2f prio=%.1f dev=%+.1f%% window=%s..%s",
			f.MeterID, f.Type, f.Severity, f.Confidence, f.PriorityScore, f.Evidence.DeviationPct,
			f.WindowStart.Format("01-02 15:04"), f.WindowEnd.Format("01-02 15:04"))
	}

	want := []struct {
		meter string
		typ   domain.AnomalyType
		sev   domain.Severity
	}{
		{"M-109", domain.AnomalyReal, domain.SeverityHigh},
		{"M-112", domain.AnomalyDataQuality, domain.SeverityHigh},
		{"M-104", domain.AnomalyExplainable, domain.SeverityMedium},
		{"M-106", domain.AnomalyFalsePositive, domain.SeverityLow},
	}
	if len(res.Findings) != len(want) {
		t.Fatalf("expected %d findings, got %d", len(want), len(res.Findings))
	}
	// Order matters: it is the investigation priority.
	for i, w := range want {
		f := res.Findings[i]
		if f.MeterID != w.meter || f.Type != w.typ || f.Severity != w.sev {
			t.Errorf("finding #%d = %s/%s/%s, want %s/%s/%s", i+1, f.MeterID, f.Type, f.Severity, w.meter, w.typ, w.sev)
		}
		if f.Confidence < 0.80 {
			t.Errorf("%s confidence %.2f, expected >= 0.80", f.MeterID, f.Confidence)
		}
	}
}

func TestDataset_RealAnomalyEvidence(t *testing.T) {
	f := findingFor(t, loadDataset(t), "M-109")
	ev := f.Evidence
	if ev.DeviationPct < 100 {
		t.Errorf("deviation %.1f%%, expected > +100%%", ev.DeviationPct)
	}
	if got := ev.Window.Start.Format("2006-01-02 15:04"); got != "2026-09-12 14:00" {
		t.Errorf("change should start at 2026-09-12 14:00, got %s", got)
	}
	if !ev.Window.Persistent {
		t.Error("the change should be persistent")
	}
	for _, e := range ev.Events {
		if e.Explains {
			t.Errorf("event %s must not explain the change", e.Type)
		}
	}
	if len(ev.Events) == 0 || ev.Events[0].Type != domain.EventUnknown {
		t.Error("the UNKNOWN record should be listed as evidence")
	}
	assertSignal(t, ev, "CURRENT_COMOVES", true)
	assertSignal(t, ev, "ELECTRICAL_CHANGE", true)
	assertSignal(t, ev, "NO_DATA_QUALITY", true)
}

func TestDataset_FalsePositiveIsExplainedByOutage(t *testing.T) {
	f := findingFor(t, loadDataset(t), "M-106")
	if f.IsAnomaly() {
		t.Error("an explained outage must not be reported as an anomaly")
	}
	if f.Evidence.Window.Persistent {
		t.Error("the outage recovered, it should not be persistent")
	}
	if f.Evidence.Window.Hours != 12 {
		t.Errorf("outage lasted %d h, expected 12", f.Evidence.Window.Hours)
	}
	assertSignal(t, f.Evidence, "DURATION_MATCH", true)
	assertSignal(t, f.Evidence, "RECOVERED", true)
}

func TestDataset_ExplainableStartsWithEvent(t *testing.T) {
	f := findingFor(t, loadDataset(t), "M-104")
	if got := f.WindowStart.Format("2006-01-02"); got != "2026-09-11" {
		t.Errorf("change should start on 2026-09-11, got %s", got)
	}
	assertSignal(t, f.Evidence, "EVENT_ALIGNED", true)
}

func TestDataset_DataQuality(t *testing.T) {
	f := findingFor(t, loadDataset(t), "M-112")
	dq := f.Evidence.DataQuality
	if dq.VoltageOutOfBand == 0 || dq.PowerInconsistency == 0 {
		t.Errorf("expected voltage and power inconsistencies, got %+v", dq)
	}
	assertSignal(t, f.Evidence, "CONSUMPTION_STABLE", true)
	assertSignal(t, f.Evidence, "DQ_EVENT_REPORTED", true)
}

func TestDataset_MeterStats(t *testing.T) {
	res := loadDataset(t)
	if len(res.Stats) != 12 {
		t.Fatalf("expected 12 meters, got %d", len(res.Stats))
	}
	for _, s := range res.Stats {
		if s.MeterID == "M-104" && (s.VariationPct < 45 || s.VariationPct > 50) {
			t.Errorf("M-104 variation %.1f%%, expected ≈ +47.6%%", s.VariationPct)
		}
	}
}

func findingFor(t *testing.T, res analytics.Result, meter string) analytics.Finding {
	t.Helper()
	for _, f := range res.Findings {
		if f.MeterID == meter {
			return f
		}
	}
	t.Fatalf("no finding for %s", meter)
	return analytics.Finding{}
}

func assertSignal(t *testing.T, ev domain.Evidence, code string, supports bool) {
	t.Helper()
	for _, s := range ev.Signals {
		if s.Code == code {
			if s.Supports != supports {
				t.Errorf("signal %s supports=%v, want %v (%s)", code, s.Supports, supports, s.Label)
			}
			return
		}
	}
	t.Errorf("signal %s not found", code)
}
