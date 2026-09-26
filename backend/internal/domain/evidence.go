package domain

import "time"

type Direction string

const (
	DirectionUp   Direction = "UP"
	DirectionDown Direction = "DOWN"
	DirectionNone Direction = "NONE"
)

// Evidence is everything the engine measured to support a finding. It is
// persisted as JSON and is the only input the explanation layer (templates or
// LLM) is allowed to use.
type Evidence struct {
	Baseline     BaselineEvidence    `json:"baseline"`
	Current      CurrentEvidence     `json:"current"`
	Window       WindowEvidence      `json:"window"`
	Direction    Direction           `json:"direction"`
	DeviationPct float64             `json:"deviation_pct"`
	Variables    []VariableChange    `json:"variables"`
	Signals      []Signal            `json:"signals"`
	Events       []EventEvidence     `json:"events"`
	DataQuality  DataQualityEvidence `json:"data_quality"`
	Confidence   ConfidenceBreakdown `json:"confidence"`
	Priority     PriorityBreakdown   `json:"priority"`
}

type BaselineEvidence struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Days     int       `json:"days"`
	DailyKWh float64   `json:"daily_kwh"`
}

type CurrentEvidence struct {
	ReferenceTime time.Time `json:"reference_time"`
	Last24hKWh    float64   `json:"last_24h_kwh"`
	VariationPct  float64   `json:"variation_pct"`
}

type WindowEvidence struct {
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	Hours      int       `json:"hours"`
	Persistent bool      `json:"persistent"`
}

type VariableChange struct {
	Name        string  `json:"name"`
	Label       string  `json:"label"`
	Unit        string  `json:"unit"`
	Before      float64 `json:"before"`
	After       float64 `json:"after"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	ChangePct   float64 `json:"change_pct"`
	ChangeAbs   float64 `json:"change_abs"`
	Significant bool    `json:"significant"`
}

type Signal struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Supports bool   `json:"supports"`
}

type EventEvidence struct {
	ID          int64     `json:"id"`
	Type        EventType `json:"type"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
	OffsetHours float64   `json:"offset_hours"`
	Explains    bool      `json:"explains"`
	Note        string    `json:"note"`
}

type DataQualityEvidence struct {
	NominalVoltage     float64 `json:"nominal_voltage"`
	VoltageOutOfBand   int     `json:"voltage_out_of_band"`
	PowerInconsistency int     `json:"power_inconsistency"`
	VoltageJumps       int     `json:"voltage_jumps"`
	MissingHours       int     `json:"missing_hours"`
	Duplicates         int     `json:"duplicates"`
	InvalidValues      int     `json:"invalid_values"`
	AffectedReadings   int     `json:"affected_readings"`
	ReadingsInWindow   int     `json:"readings_in_window"`
	AffectedPct        float64 `json:"affected_pct"`
}

type ConfidenceBreakdown struct {
	Magnitude     float64 `json:"magnitude"`
	Corroboration float64 `json:"corroboration"`
	Context       float64 `json:"context"`
	Value         float64 `json:"value"`
}

type PriorityBreakdown struct {
	Severity  float64 `json:"severity"`
	Type      float64 `json:"type"`
	Magnitude float64 `json:"magnitude"`
	Recency   float64 `json:"recency"`
	Total     float64 `json:"total"`
}
