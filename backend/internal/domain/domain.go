package domain

import "time"

type Meter struct {
	ID        int64       `json:"id"`
	MeterID   string      `json:"meter_id"`
	Name      string      `json:"name"`
	Location  string      `json:"location"`
	Status    MeterStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}

type MeterStatus string

const (
	MeterOK       MeterStatus = "OK"
	MeterAlert    MeterStatus = "ALERT"
	MeterCritical MeterStatus = "CRITICAL"
)

func (s MeterStatus) Rank() int {
	switch s {
	case MeterCritical:
		return 3
	case MeterAlert:
		return 2
	default:
		return 1
	}
}

type Reading struct {
	MeterID        string    `json:"meter_id"`
	Timestamp      time.Time `json:"timestamp"`
	ConsumptionKWh float64   `json:"consumption_kwh"`
	VoltageV       float64   `json:"voltage_v"`
	CurrentA       float64   `json:"current_a"`
	PowerFactor    float64   `json:"power_factor"`
	Status         string    `json:"status"`
}

type EventType string

const (
	EventOperationalChange EventType = "OPERATIONAL_CHANGE"
	EventScheduledOutage   EventType = "SCHEDULED_OUTAGE"
	EventDataQuality       EventType = "DATA_QUALITY"
	EventUnknown           EventType = "UNKNOWN"
)

type Event struct {
	ID          int64     `json:"id"`
	MeterID     string    `json:"meter_id"`
	Timestamp   time.Time `json:"timestamp"`
	Type        EventType `json:"type"`
	Description string    `json:"description"`
}

type AnomalyType string

const (
	AnomalyReal          AnomalyType = "REAL_ANOMALY"
	AnomalyExplainable   AnomalyType = "EXPLAINABLE_ANOMALY"
	AnomalyDataQuality   AnomalyType = "DATA_QUALITY"
	AnomalyFalsePositive AnomalyType = "FALSE_POSITIVE"
)

func (t AnomalyType) Valid() bool {
	switch t {
	case AnomalyReal, AnomalyExplainable, AnomalyDataQuality, AnomalyFalsePositive:
		return true
	}
	return false
}

type Severity string

const (
	SeverityHigh   Severity = "HIGH"
	SeverityMedium Severity = "MEDIUM"
	SeverityLow    Severity = "LOW"
)

func (s Severity) Valid() bool {
	return s == SeverityHigh || s == SeverityMedium || s == SeverityLow
}

func (s Severity) Rank() int {
	switch s {
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	}
	return 0
}

type AnomalyStatus string

const (
	StatusOpen          AnomalyStatus = "OPEN"
	StatusInvestigating AnomalyStatus = "INVESTIGATING"
	StatusResolved      AnomalyStatus = "RESOLVED"
	StatusDismissed     AnomalyStatus = "DISMISSED"
)

func (s AnomalyStatus) Valid() bool {
	switch s {
	case StatusOpen, StatusInvestigating, StatusResolved, StatusDismissed:
		return true
	}
	return false
}

func (s AnomalyStatus) Active() bool {
	return s == StatusOpen || s == StatusInvestigating
}

type Anomaly struct {
	ID                int64         `json:"id"`
	AnalysisID        string        `json:"analysis_id"`
	MeterID           string        `json:"meter_id"`
	DetectedAt        time.Time     `json:"detected_at"`
	Type              AnomalyType   `json:"type"`
	IsAnomaly         bool          `json:"anomaly"`
	Severity          Severity      `json:"severity"`
	Confidence        float64       `json:"confidence"`
	PriorityScore     float64       `json:"priority_score"`
	Reason            string        `json:"reason"`
	RecommendedAction string        `json:"recommended_action"`
	EvidenceSummary   []string      `json:"evidence_summary"`
	Status            AnomalyStatus `json:"status"`
	Note              string        `json:"note"`
	WindowStart       time.Time     `json:"window_start"`
	WindowEnd         time.Time     `json:"window_end"`
	Evidence          Evidence      `json:"evidence"`
	ExplanationSource string        `json:"explanation_source"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type RunStatus string

const (
	RunPending   RunStatus = "PENDING"
	RunRunning   RunStatus = "RUNNING"
	RunCompleted RunStatus = "COMPLETED"
	RunFailed    RunStatus = "FAILED"
)

type StepState string

const (
	StepPending StepState = "PENDING"
	StepRunning StepState = "RUNNING"
	StepDone    StepState = "DONE"
	StepFailed  StepState = "FAILED"
)

const (
	StepReadings       = "READINGS"
	StepBaseline       = "BASELINE"
	StepDetection      = "DETECTION"
	StepCorrelation    = "CORRELATION"
	StepEvents         = "EVENTS"
	StepExplanation    = "EXPLANATION"
	StepRecommendation = "RECOMMENDATION"
)

type AnalysisStep struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	State     StepState  `json:"state"`
	Detail    string     `json:"detail,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

func NewAnalysisSteps() []AnalysisStep {
	return []AnalysisStep{
		{Name: StepReadings, Label: "Lecturas", State: StepPending},
		{Name: StepBaseline, Label: "Baseline", State: StepPending},
		{Name: StepDetection, Label: "Detección", State: StepPending},
		{Name: StepCorrelation, Label: "Correlación", State: StepPending},
		{Name: StepEvents, Label: "Eventos", State: StepPending},
		{Name: StepExplanation, Label: "Explicación", State: StepPending},
		{Name: StepRecommendation, Label: "Recomendación", State: StepPending},
	}
}

type AnalysisSummary struct {
	MetersAnalyzed    int            `json:"meters_analyzed"`
	ReadingsAnalyzed  int            `json:"readings_analyzed"`
	Detected          int            `json:"detected"`
	HighPriority      int            `json:"high_priority"`
	ByType            map[string]int `json:"by_type"`
	AvgConfidence     float64        `json:"avg_confidence"`
	ExplanationSource string         `json:"explanation_source"`
	Text              string         `json:"text"`
}

type AnalysisRun struct {
	ID          string           `json:"id"`
	Status      RunStatus        `json:"status"`
	CurrentStep string           `json:"current_step"`
	Steps       []AnalysisStep   `json:"steps"`
	Summary     *AnalysisSummary `json:"summary"`
	Error       string           `json:"error,omitempty"`
	StartedAt   time.Time        `json:"started_at"`
	FinishedAt  *time.Time       `json:"finished_at,omitempty"`
}

type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
}

// MeterStatusFor derives the status of a meter from its findings. Only
// active findings count, so resolving or dismissing an anomaly clears the
// alert:
//   - CRITICAL: an active real anomaly of HIGH severity,
//   - ALERT: any other active anomaly of HIGH or MEDIUM severity,
//   - OK: otherwise (false positives never raise an alert).
func MeterStatusFor(anomalies []Anomaly) MeterStatus {
	status := MeterOK
	for _, a := range anomalies {
		if !a.IsAnomaly || !a.Status.Active() {
			continue
		}
		switch {
		case a.Type == AnomalyReal && a.Severity == SeverityHigh:
			return MeterCritical
		case a.Severity == SeverityHigh || a.Severity == SeverityMedium:
			status = MeterAlert
		}
	}
	return status
}
