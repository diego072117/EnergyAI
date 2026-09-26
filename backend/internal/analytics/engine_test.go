package analytics

import (
	"math"
	"testing"
	"time"

	"energyai/internal/domain"
)

var t0 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// series builds 14 days of hourly readings with a daily profile and a small
// deterministic wobble. mutate may alter each reading (index = hour offset).
func series(meter string, mutate func(i int, r *domain.Reading)) []domain.Reading {
	var out []domain.Reading
	for i := 0; i < 14*24; i++ {
		h := i % 24
		profile := 1 + 0.4*math.Sin(float64(h)/24*2*math.Pi)
		wobble := 1 + 0.03*math.Sin(float64(i)*1.7)
		kwh := 40 * profile * wobble
		pf := 0.94
		v := 220.0 + math.Sin(float64(i))
		cur := kwh * 1000 / (v * pf) / 0.95 // keeps V·I·PF/kWh stable ≈ 1.05
		r := domain.Reading{MeterID: meter, Timestamp: t0.Add(time.Duration(i) * time.Hour),
			ConsumptionKWh: kwh, VoltageV: v, CurrentA: cur, PowerFactor: pf, Status: "OK"}
		if mutate != nil {
			mutate(i, &r)
		}
		out = append(out, r)
	}
	return out
}

const day = 24

func run(t *testing.T, rs []domain.Reading, events ...domain.Event) []Finding {
	t.Helper()
	return Run(DefaultConfig(), []MeterInput{{MeterID: "X", Readings: rs, Events: events}}).Findings
}

func TestStableMeterHasNoFindings(t *testing.T) {
	if fs := run(t, series("X", nil)); len(fs) != 0 {
		t.Fatalf("expected no findings, got %+v", fs)
	}
}

func TestPersistentIncreaseWithoutEventIsRealAnomaly(t *testing.T) {
	rs := series("X", func(i int, r *domain.Reading) {
		if i >= 11*day+14 {
			r.ConsumptionKWh *= 2
			r.CurrentA *= 2
			r.PowerFactor = 0.74
		}
	})
	fs := run(t, rs)
	if len(fs) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(fs))
	}
	f := fs[0]
	if f.Type != domain.AnomalyReal || f.Severity != domain.SeverityHigh {
		t.Errorf("got %s/%s", f.Type, f.Severity)
	}
	if !f.WindowStart.Equal(t0.Add((11*day + 14) * time.Hour)) {
		t.Errorf("start refined to %s", f.WindowStart)
	}
	if !f.Evidence.Window.Persistent {
		t.Error("should be persistent")
	}
}

func TestUnknownEventNeverExplains(t *testing.T) {
	start := t0.Add((11*day + 14) * time.Hour)
	rs := series("X", func(i int, r *domain.Reading) {
		if i >= 11*day+14 {
			r.ConsumptionKWh *= 2
			r.CurrentA *= 2
		}
	})
	fs := run(t, rs, domain.Event{MeterID: "X", Timestamp: start, Type: domain.EventUnknown, Description: "No operational event reported"})
	if fs[0].Type != domain.AnomalyReal {
		t.Fatalf("UNKNOWN must not explain a change, got %s", fs[0].Type)
	}
	if fs[0].Evidence.Confidence.Context != 1 {
		t.Errorf("explicit record should raise context to 1, got %v", fs[0].Evidence.Confidence.Context)
	}
}

func TestOperationalChangeExplainsIncrease(t *testing.T) {
	start := t0.Add(10 * day * time.Hour)
	rs := series("X", func(i int, r *domain.Reading) {
		if i >= 10*day {
			r.ConsumptionKWh *= 1.5
			r.CurrentA *= 1.5
		}
	})
	fs := run(t, rs, domain.Event{MeterID: "X", Timestamp: start, Type: domain.EventOperationalChange, Description: "New line"})
	if len(fs) != 1 || fs[0].Type != domain.AnomalyExplainable || fs[0].Severity != domain.SeverityMedium {
		t.Fatalf("got %+v", fs)
	}
}

func TestScheduledOutageIsFalsePositive(t *testing.T) {
	start := t0.Add(8 * day * time.Hour)
	rs := series("X", func(i int, r *domain.Reading) {
		if i >= 8*day && i < 8*day+12 {
			r.ConsumptionKWh *= 0.2
			r.CurrentA *= 0.2
		}
	})
	fs := run(t, rs, domain.Event{MeterID: "X", Timestamp: start, Type: domain.EventScheduledOutage, Description: "Maintenance for 12 hours"})
	if len(fs) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(fs))
	}
	f := fs[0]
	if f.Type != domain.AnomalyFalsePositive || f.Severity != domain.SeverityLow || f.IsAnomaly() {
		t.Errorf("got %s/%s", f.Type, f.Severity)
	}
	if f.Evidence.Window.Hours != 12 || f.Evidence.Window.Persistent {
		t.Errorf("window %+v", f.Evidence.Window)
	}
}

func TestOutageEventDoesNotExplainIncrease(t *testing.T) {
	start := t0.Add(10 * day * time.Hour)
	rs := series("X", func(i int, r *domain.Reading) {
		if i >= 10*day {
			r.ConsumptionKWh *= 1.8
			r.CurrentA *= 1.8
		}
	})
	fs := run(t, rs, domain.Event{MeterID: "X", Timestamp: start, Type: domain.EventScheduledOutage, Description: "Outage"})
	if fs[0].Type != domain.AnomalyReal {
		t.Fatalf("an outage cannot explain an increase, got %s", fs[0].Type)
	}
}

func TestInconsistentElectricalReadingsAreDataQuality(t *testing.T) {
	rs := series("X", func(i int, r *domain.Reading) {
		if i >= 12*day && i%3 == 0 {
			if (i/3)%2 == 0 {
				r.VoltageV = 241
			} else {
				r.VoltageV = 202
			}
			r.PowerFactor = 0.58
		}
	})
	fs := run(t, rs, domain.Event{MeterID: "X", Timestamp: t0.Add(12 * day * time.Hour), Type: domain.EventDataQuality, Description: "Intermittent readings"})
	if len(fs) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(fs))
	}
	f := fs[0]
	if f.Type != domain.AnomalyDataQuality || f.Severity != domain.SeverityHigh {
		t.Errorf("got %s/%s", f.Type, f.Severity)
	}
	if f.Evidence.Confidence.Context != 1 {
		t.Error("reported DATA_QUALITY event should raise context")
	}
}

func TestSmallNoiseDoesNotTriggerSpikes(t *testing.T) {
	// ±20% single-hour deviations: z-score is high (tiny MAD) but the
	// relative deviation guard must keep them out.
	rs := series("X", func(i int, r *domain.Reading) {
		if i >= 8*day && i%17 == 0 {
			r.ConsumptionKWh *= 1.2
		}
	})
	if fs := run(t, rs); len(fs) != 0 {
		t.Fatalf("noise should not be reported, got %d findings", len(fs))
	}
}

func TestIsolatedSpikeIsReported(t *testing.T) {
	rs := series("X", func(i int, r *domain.Reading) {
		if i == 10*day+5 {
			r.ConsumptionKWh *= 2.5
		}
	})
	fs := run(t, rs)
	if len(fs) != 1 || fs[0].Type != domain.AnomalyReal || fs[0].Severity != domain.SeverityMedium {
		t.Fatalf("expected one MEDIUM real spike, got %+v", fs)
	}
}

func TestShortDeviationBelowMinHoursIsIgnoredAsShift(t *testing.T) {
	segs := DetectLevelShifts(series("X", func(i int, r *domain.Reading) {
		if i >= 9*day && i < 9*day+3 {
			r.ConsumptionKWh *= 1.4
		}
	})[7*day:], mustBaseline(t, series("X", nil)), DefaultConfig())
	if len(segs) != 0 {
		t.Fatalf("3h deviation should not be a level shift, got %d", len(segs))
	}
}

func TestValidateReadings(t *testing.T) {
	rs := series("X", nil)[:10]
	rs = append(rs, rs[3])                                                                              // duplicate
	rs = append(rs[:5], rs[6:]...)                                                                      // gap of 1h
	rs = append(rs, domain.Reading{Timestamp: t0.Add(20 * time.Hour), PowerFactor: 1.4, VoltageV: 220}) // invalid PF
	clean, rep := ValidateReadings(rs)
	if rep.Duplicates != 1 || rep.InvalidValues != 1 || rep.MissingHours != 1 {
		t.Errorf("report = %+v", rep)
	}
	for i := 1; i < len(clean); i++ {
		if !clean[i].Timestamp.After(clean[i-1].Timestamp) {
			t.Fatal("clean readings must be strictly increasing")
		}
	}
}

func TestSplitBaselineRequiresAnalysisData(t *testing.T) {
	if _, _, err := SplitBaseline(series("X", nil)[:5*day], 7); err == nil {
		t.Fatal("expected error when there is no data after the baseline")
	}
}

func TestBaselineDailyKWh(t *testing.T) {
	b := mustBaseline(t, series("X", nil))
	if b.DailyKWh < 900 || b.DailyKWh > 1020 {
		t.Errorf("daily baseline %.1f out of expected range", b.DailyKWh)
	}
	if math.Abs(b.PowerRatio-1.0526) > 0.01 {
		t.Errorf("power ratio %.4f", b.PowerRatio)
	}
}

func TestConfidenceIsBoundedAndMonotonic(t *testing.T) {
	if c := Confidence(1, 1, 1); c != 0.98 {
		t.Errorf("max confidence must be capped at 0.98, got %v", c)
	}
	if Confidence(0.2, 0.5, 0.5) >= Confidence(0.8, 0.5, 0.5) {
		t.Error("more magnitude must mean more confidence")
	}
	if c := Confidence(0, 0, 0); c != 0.40 {
		t.Errorf("min confidence = %v", c)
	}
}

func TestPriorityOrdering(t *testing.T) {
	real := Priority(domain.SeverityHigh, domain.AnomalyReal, 0.7, true).Total
	dq := Priority(domain.SeverityHigh, domain.AnomalyDataQuality, 0.7, true).Total
	expl := Priority(domain.SeverityMedium, domain.AnomalyExplainable, 1, true).Total
	fp := Priority(domain.SeverityLow, domain.AnomalyFalsePositive, 1, false).Total
	if !(real > dq && dq > expl && expl > fp) {
		t.Errorf("unexpected ordering real=%v dq=%v expl=%v fp=%v", real, dq, expl, fp)
	}
}

func TestEventDuration(t *testing.T) {
	cases := map[string]int{"Scheduled maintenance outage for 12 hours": 12, "parada de 8 horas": 8, "6h window": 6}
	for s, want := range cases {
		if got, ok := eventDuration(s); !ok || got != want {
			t.Errorf("eventDuration(%q) = %d,%v", s, got, ok)
		}
	}
	if _, ok := eventDuration("New production line activated"); ok {
		t.Error("no duration expected")
	}
}

func TestBuildInputsIgnoresEventsWithoutReadings(t *testing.T) {
	in := BuildInputs(series("A", nil)[:2], []domain.Event{{MeterID: "A"}, {MeterID: "Z"}})
	if len(in) != 1 || in[0].MeterID != "A" || len(in[0].Events) != 1 {
		t.Fatalf("got %+v", in)
	}
}

func TestSnapshotVariation(t *testing.T) {
	rs := series("X", func(i int, r *domain.Reading) {
		if i >= 13*day {
			r.ConsumptionKWh *= 1.5
		}
	})
	st, _, _, err := Snapshot("X", rs, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if st.VariationPct < 45 || st.VariationPct > 55 {
		t.Errorf("variation %.1f%%, expected ≈ +50%%", st.VariationPct)
	}
}

func mustBaseline(t *testing.T, rs []domain.Reading) Baseline {
	t.Helper()
	clean, _ := ValidateReadings(rs)
	b, _, err := SplitBaseline(clean, 7)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
