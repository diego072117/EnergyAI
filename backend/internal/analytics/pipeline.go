package analytics

import (
	"fmt"
	"sort"
	"time"

	"energyai/internal/domain"
	"energyai/internal/format"
)

type MeterInput struct {
	MeterID  string
	Readings []domain.Reading
	Events   []domain.Event
}

type MeterStats struct {
	MeterID          string           `json:"meter_id"`
	Readings         int              `json:"readings"`
	PeriodStart      time.Time        `json:"period_start"`
	PeriodEnd        time.Time        `json:"period_end"`
	TotalKWh         float64          `json:"total_kwh"`
	BaselineDailyKWh float64          `json:"baseline_daily_kwh"`
	Last24hKWh       float64          `json:"last_24h_kwh"`
	VariationPct     float64          `json:"variation_pct"`
	AvgVoltage24h    float64          `json:"avg_voltage_24h"`
	AvgCurrent24h    float64          `json:"avg_current_24h"`
	AvgPF24h         float64          `json:"avg_power_factor_24h"`
	Validation       ValidationReport `json:"validation"`
}

func Snapshot(meterID string, rs []domain.Reading, cfg Config) (MeterStats, Baseline, []domain.Reading, error) {
	clean, rep := ValidateReadings(rs)
	b, analysis, err := SplitBaseline(clean, cfg.BaselineDays)
	if err != nil {
		return MeterStats{MeterID: meterID, Validation: rep}, Baseline{}, nil, err
	}
	return computeStats(meterID, clean, rep, b), b, analysis, nil
}

func computeStats(meterID string, clean []domain.Reading, rep ValidationReport, b Baseline) MeterStats {
	st := MeterStats{
		MeterID:          meterID,
		Readings:         len(clean),
		PeriodStart:      clean[0].Timestamp,
		PeriodEnd:        clean[len(clean)-1].Timestamp,
		BaselineDailyKWh: round(b.DailyKWh, 1),
		Validation:       rep,
	}
	cut := st.PeriodEnd.Add(-24 * time.Hour)
	var v, c, pf []float64
	for _, r := range clean {
		st.TotalKWh += r.ConsumptionKWh
		if r.Timestamp.After(cut) {
			st.Last24hKWh += r.ConsumptionKWh
			v = append(v, r.VoltageV)
			c = append(c, r.CurrentA)
			pf = append(pf, r.PowerFactor)
		}
	}
	st.TotalKWh = round(st.TotalKWh, 1)
	st.Last24hKWh = round(st.Last24hKWh, 1)
	st.VariationPct = round(relChange(b.DailyKWh, st.Last24hKWh)*100, 1)
	st.AvgVoltage24h = round(mean(v), 1)
	st.AvgCurrent24h = round(mean(c), 1)
	st.AvgPF24h = round(mean(pf), 3)
	return st
}

type meterState struct {
	in         MeterInput
	clean      []domain.Reading
	validation ValidationReport
	baseline   Baseline
	analysis   []domain.Reading
	stats      MeterStats
	segments   []Segment
	spikes     []Spike
	dq         DQResult
	segElec    []Electrical
	segEvents  [][]domain.EventEvidence
	err        error
}

type Pipeline struct {
	cfg    Config
	meters []*meterState
}

func NewPipeline(cfg Config, inputs []MeterInput) *Pipeline {
	p := &Pipeline{cfg: cfg}
	for _, in := range inputs {
		p.meters = append(p.meters, &meterState{in: in})
	}
	sort.Slice(p.meters, func(i, j int) bool { return p.meters[i].in.MeterID < p.meters[j].in.MeterID })
	return p
}

func (p *Pipeline) active() []*meterState {
	var out []*meterState
	for _, m := range p.meters {
		if m.err == nil {
			out = append(out, m)
		}
	}
	return out
}

func (p *Pipeline) Validate() string {
	total, issues := 0, 0
	for _, m := range p.meters {
		m.clean, m.validation = ValidateReadings(m.in.Readings)
		total += m.validation.Total
		issues += m.validation.Duplicates + m.validation.InvalidValues + m.validation.MissingHours
	}
	return fmt.Sprintf("%s lecturas de %d medidores validadas (%d incidencias estructurales)",
		format.Number(float64(total), 0), len(p.meters), issues)
}

func (p *Pipeline) ComputeBaselines() string {
	for _, m := range p.meters {
		b, analysis, err := SplitBaseline(m.clean, p.cfg.BaselineDays)
		if err != nil {
			m.err = err
			continue
		}
		m.baseline, m.analysis = b, analysis
		m.stats = computeStats(m.in.MeterID, m.clean, m.validation, b)
	}
	msg := fmt.Sprintf("Perfil horario de %d días calculado para %d medidores", p.cfg.BaselineDays, len(p.active()))
	if skipped := len(p.meters) - len(p.active()); skipped > 0 {
		msg += fmt.Sprintf(" (%d sin datos suficientes)", skipped)
	}
	return msg
}

func (p *Pipeline) Detect() string {
	shifts, spikes, dq := 0, 0, 0
	for _, m := range p.active() {
		m.segments = DetectLevelShifts(m.analysis, m.baseline, p.cfg)
		m.spikes = DetectSpikes(m.analysis, m.baseline, m.segments, p.cfg)
		m.dq = DetectDataQuality(m.analysis, m.baseline, m.validation, p.cfg)
		shifts += len(m.segments)
		spikes += len(m.spikes)
		if m.dq.Signals >= p.cfg.DQMinSignals {
			dq++
		}
	}
	return fmt.Sprintf("%d %s, %d %s, %d %s con señales de calidad de datos",
		shifts, plural(shifts, "cambio de nivel", "cambios de nivel"),
		spikes, plural(spikes, "pico aislado", "picos aislados"),
		dq, plural(dq, "medidor", "medidores"))
}

func (p *Pipeline) Correlate() string {
	coherent := 0
	for _, m := range p.active() {
		m.segElec = make([]Electrical, len(m.segments))
		for i, s := range m.segments {
			m.segElec[i] = AnalyzeElectrical(m.analysis, s.StartIdx, s.EndIdx, m.baseline, p.cfg)
			if m.segElec[i].CurrentCoMoves {
				coherent++
			}
		}
	}
	return fmt.Sprintf("Relación consumo↔corriente↔voltaje↔FP evaluada: %d %s",
		coherent, plural(coherent, "cambio eléctricamente coherente", "cambios eléctricamente coherentes"))
}

func (p *Pipeline) MatchEvents() string {
	related, explaining := 0, 0
	for _, m := range p.active() {
		m.segEvents = make([][]domain.EventEvidence, len(m.segments))
		for i, s := range m.segments {
			m.segEvents[i] = MatchSegmentEvents(m.in.Events, s, p.cfg)
			related += len(m.segEvents[i])
			if firstExplaining(m.segEvents[i]) != nil {
				explaining++
			}
		}
	}
	return fmt.Sprintf("%d %s con cambios; %d %s por la operación",
		related, plural(related, "evento relacionado", "eventos relacionados"),
		explaining, plural(explaining, "cambio explicado", "cambios explicados"))
}

func (p *Pipeline) Findings() []Finding {
	var out []Finding
	for _, m := range p.active() {
		out = append(out, m.findings(p.cfg)...)
	}
	SortFindings(out)
	return out
}

func (p *Pipeline) Stats() []MeterStats {
	var out []MeterStats
	for _, m := range p.active() {
		out = append(out, m.stats)
	}
	return out
}

func (p *Pipeline) Skipped() map[string]error {
	out := map[string]error{}
	for _, m := range p.meters {
		if m.err != nil {
			out[m.in.MeterID] = m.err
		}
	}
	return out
}

type Result struct {
	Findings []Finding
	Stats    []MeterStats
}

func Run(cfg Config, inputs []MeterInput) Result {
	p := NewPipeline(cfg, inputs)
	p.Validate()
	p.ComputeBaselines()
	p.Detect()
	p.Correlate()
	p.MatchEvents()
	return Result{Findings: p.Findings(), Stats: p.Stats()}
}

func BuildInputs(readings []domain.Reading, events []domain.Event) []MeterInput {
	idx := map[string]int{}
	var out []MeterInput
	for _, r := range readings {
		i, ok := idx[r.MeterID]
		if !ok {
			i = len(out)
			idx[r.MeterID] = i
			out = append(out, MeterInput{MeterID: r.MeterID})
		}
		out[i].Readings = append(out[i].Readings, r)
	}
	for _, e := range events {
		if i, ok := idx[e.MeterID]; ok {
			out[i].Events = append(out[i].Events, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MeterID < out[j].MeterID })
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
