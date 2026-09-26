package domain

import "testing"

func TestMeterStatusFor(t *testing.T) {
	real := Anomaly{Type: AnomalyReal, Severity: SeverityHigh, IsAnomaly: true, Status: StatusOpen}
	dq := Anomaly{Type: AnomalyDataQuality, Severity: SeverityHigh, IsAnomaly: true, Status: StatusOpen}
	fp := Anomaly{Type: AnomalyFalsePositive, Severity: SeverityLow, IsAnomaly: false, Status: StatusOpen}
	lowReal := Anomaly{Type: AnomalyReal, Severity: SeverityLow, IsAnomaly: true, Status: StatusOpen}
	resolved := real
	resolved.Status = StatusResolved

	cases := []struct {
		name string
		in   []Anomaly
		want MeterStatus
	}{
		{"none", nil, MeterOK},
		{"real high", []Anomaly{dq, real}, MeterCritical},
		{"data quality", []Anomaly{dq}, MeterAlert},
		{"false positive", []Anomaly{fp}, MeterOK},
		{"low severity", []Anomaly{lowReal}, MeterOK},
		{"resolved", []Anomaly{resolved}, MeterOK},
	}
	for _, c := range cases {
		if got := MeterStatusFor(c.in); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestEnumsValid(t *testing.T) {
	if !AnomalyReal.Valid() || AnomalyType("X").Valid() {
		t.Error("AnomalyType.Valid")
	}
	if !SeverityLow.Valid() || Severity("X").Valid() {
		t.Error("Severity.Valid")
	}
	if !StatusDismissed.Valid() || AnomalyStatus("X").Valid() {
		t.Error("AnomalyStatus.Valid")
	}
	if SeverityHigh.Rank() <= SeverityMedium.Rank() || MeterCritical.Rank() <= MeterAlert.Rank() {
		t.Error("Rank ordering")
	}
	if len(NewAnalysisSteps()) != 7 {
		t.Error("expected 7 pipeline steps")
	}
}

func TestAnalysisRunCloneIsIndependent(t *testing.T) {
	orig := AnalysisRun{Steps: NewAnalysisSteps(), Summary: &AnalysisSummary{ByType: map[string]int{"A": 1}}}
	c := orig.Clone()
	c.Steps[0].State = StepDone
	c.Summary.ByType["A"] = 2
	if orig.Steps[0].State != StepPending || orig.Summary.ByType["A"] != 1 {
		t.Fatal("clone must not share steps or summary with the original")
	}
	if (AnalysisRun{}).Clone().Summary != nil {
		t.Error("nil summary must stay nil")
	}
}
