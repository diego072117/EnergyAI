package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/store"
)

type MeterService struct {
	repo Repository
	cfg  analytics.Config
}

func NewMeterService(repo Repository, cfg analytics.Config) *MeterService {
	return &MeterService{repo: repo, cfg: cfg}
}

type AnomalyRef struct {
	ID            int64                `json:"id"`
	Type          domain.AnomalyType   `json:"type"`
	Severity      domain.Severity      `json:"severity"`
	Confidence    float64              `json:"confidence"`
	PriorityScore float64              `json:"priority_score"`
	IsAnomaly     bool                 `json:"anomaly"`
	Status        domain.AnomalyStatus `json:"status"`
}

type MeterSummary struct {
	MeterID          string             `json:"meter_id"`
	Name             string             `json:"name"`
	Location         string             `json:"location"`
	Status           domain.MeterStatus `json:"status"`
	TotalKWh         float64            `json:"total_kwh"`
	BaselineDailyKWh float64            `json:"baseline_daily_kwh"`
	Last24hKWh       float64            `json:"last_24h_kwh"`
	VariationPct     float64            `json:"variation_pct"`
	Anomaly          *AnomalyRef        `json:"anomaly"`
}

type MeterFilter struct {
	Status string // all | ok | alert | critical
	Query  string
	Sort   string // meter | consumption | variation | severity
	Order  string // asc | desc
}

func (f *MeterFilter) Validate() error {
	f.Status, f.Sort, f.Order = strings.ToLower(f.Status), strings.ToLower(f.Sort), strings.ToLower(f.Order)
	if f.Status == "" {
		f.Status = "all"
	}
	if f.Sort == "" {
		f.Sort = "severity"
	}
	if f.Order == "" {
		f.Order = "desc"
		if f.Sort == "meter" {
			f.Order = "asc"
		}
	}
	switch {
	case !oneOf(f.Status, "all", "ok", "alert", "critical"):
		return fmt.Errorf("%w: status must be all, ok, alert or critical", ErrInvalidInput)
	case !oneOf(f.Sort, "meter", "consumption", "variation", "severity"):
		return fmt.Errorf("%w: sort must be meter, consumption, variation or severity", ErrInvalidInput)
	case !oneOf(f.Order, "asc", "desc"):
		return fmt.Errorf("%w: order must be asc or desc", ErrInvalidInput)
	}
	return nil
}

func oneOf(v string, opts ...string) bool {
	for _, o := range opts {
		if v == o {
			return true
		}
	}
	return false
}

type meterContext struct {
	meters    []domain.Meter
	stats     map[string]analytics.MeterStats
	baselines map[string]analytics.Baseline
	anomalies map[string][]domain.Anomaly
}

func (s *MeterService) load(ctx context.Context, meterID string) (meterContext, error) {
	mc := meterContext{stats: map[string]analytics.MeterStats{}, baselines: map[string]analytics.Baseline{}, anomalies: map[string][]domain.Anomaly{}}
	if meterID == "" {
		ms, err := s.repo.ListMeters(ctx)
		if err != nil {
			return mc, err
		}
		mc.meters = ms
	} else {
		m, err := s.repo.GetMeter(ctx, meterID)
		if err != nil {
			return mc, err
		}
		mc.meters = []domain.Meter{m}
	}
	readings, err := s.repo.ListReadings(ctx, store.ReadingsQuery{MeterID: meterID})
	if err != nil {
		return mc, err
	}
	for _, in := range analytics.BuildInputs(readings, nil) {
		st, b, _, err := analytics.Snapshot(in.MeterID, in.Readings, s.cfg)
		if err == nil {
			mc.stats[in.MeterID], mc.baselines[in.MeterID] = st, b
		}
	}
	_, as, err := latestAnomalies(ctx, s.repo, store.AnomalyFilter{MeterID: meterID})
	if err != nil {
		return mc, err
	}
	for _, a := range as {
		mc.anomalies[a.MeterID] = append(mc.anomalies[a.MeterID], a)
	}
	return mc, nil
}

func summarize(m domain.Meter, st analytics.MeterStats, as []domain.Anomaly) MeterSummary {
	sum := MeterSummary{
		MeterID: m.MeterID, Name: m.Name, Location: m.Location, Status: m.Status,
		TotalKWh: st.TotalKWh, BaselineDailyKWh: st.BaselineDailyKWh,
		Last24hKWh: st.Last24hKWh, VariationPct: st.VariationPct,
	}
	if len(as) > 0 { // anomalies are sorted by priority
		a := as[0]
		sum.Anomaly = &AnomalyRef{ID: a.ID, Type: a.Type, Severity: a.Severity, Confidence: a.Confidence,
			PriorityScore: a.PriorityScore, IsAnomaly: a.IsAnomaly, Status: a.Status}
	}
	return sum
}

func (s *MeterService) List(ctx context.Context, f MeterFilter) ([]MeterSummary, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	mc, err := s.load(ctx, "")
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	out := []MeterSummary{}
	for _, m := range mc.meters {
		if f.Status != "all" && strings.ToLower(string(m.Status)) != f.Status {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(m.MeterID+" "+m.Name+" "+m.Location), q) {
			continue
		}
		out = append(out, summarize(m, mc.stats[m.MeterID], mc.anomalies[m.MeterID]))
	}
	sortMeters(out, f.Sort, f.Order == "desc")
	return out, nil
}

func severityKey(m MeterSummary) float64 {
	k := float64(m.Status.Rank()) * 1000
	if m.Anomaly != nil {
		k += m.Anomaly.PriorityScore
	}
	return k
}

func sortMeters(ms []MeterSummary, by string, desc bool) {
	key := func(m MeterSummary) float64 {
		switch by {
		case "consumption":
			return m.Last24hKWh
		case "variation":
			return m.VariationPct
		case "severity":
			return severityKey(m)
		}
		return 0
	}
	sort.SliceStable(ms, func(i, j int) bool {
		if by == "meter" {
			if desc {
				return ms[i].MeterID > ms[j].MeterID
			}
			return ms[i].MeterID < ms[j].MeterID
		}
		ki, kj := key(ms[i]), key(ms[j])
		if ki == kj {
			return ms[i].MeterID < ms[j].MeterID
		}
		if desc {
			return ki > kj
		}
		return ki < kj
	})
}

type BaselineInfo struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Days     int       `json:"days"`
	DailyKWh float64   `json:"daily_kwh"`
}

type MeterDetail struct {
	MeterSummary
	CreatedAt time.Time            `json:"created_at"`
	Stats     analytics.MeterStats `json:"stats"`
	Baseline  *BaselineInfo        `json:"baseline"`
	Anomalies []domain.Anomaly     `json:"anomalies"`
	Events    []domain.Event       `json:"events"`
}

func (s *MeterService) Get(ctx context.Context, meterID string) (MeterDetail, error) {
	mc, err := s.load(ctx, meterID)
	if err != nil {
		return MeterDetail{}, err
	}
	m := mc.meters[0]
	events, err := s.repo.ListEvents(ctx, meterID)
	if err != nil {
		return MeterDetail{}, err
	}
	d := MeterDetail{
		MeterSummary: summarize(m, mc.stats[meterID], mc.anomalies[meterID]),
		CreatedAt:    m.CreatedAt,
		Stats:        mc.stats[meterID],
		Anomalies:    nonNilAnomalies(mc.anomalies[meterID]),
		Events:       events,
	}
	if d.Events == nil {
		d.Events = []domain.Event{}
	}
	if b, ok := mc.baselines[meterID]; ok {
		d.Baseline = &BaselineInfo{From: b.From, To: b.To, Days: b.Days, DailyKWh: d.Stats.BaselineDailyKWh}
	}
	return d, nil
}

func nonNilAnomalies(as []domain.Anomaly) []domain.Anomaly {
	if as == nil {
		return []domain.Anomaly{}
	}
	return as
}

type ReadingsParams struct {
	From       *time.Time
	To         *time.Time
	Resolution string // hour | day
}

type ReadingPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	ConsumptionKWh float64   `json:"consumption_kwh"`
	VoltageV       float64   `json:"voltage_v"`
	CurrentA       float64   `json:"current_a"`
	PowerFactor    float64   `json:"power_factor"`
	ExpectedKWh    float64   `json:"expected_kwh"`
	LowerKWh       float64   `json:"lower_kwh"`
	UpperKWh       float64   `json:"upper_kwh"`
	ExpectedV      float64   `json:"expected_voltage_v"`
	ExpectedA      float64   `json:"expected_current_a"`
	ExpectedPF     float64   `json:"expected_power_factor"`
}

type Thresholds struct {
	ShiftPct            float64 `json:"shift_pct"`
	NominalVoltage      float64 `json:"nominal_voltage"`
	VoltageTolerancePct float64 `json:"voltage_tolerance_pct"`
}

type ReadingSeries struct {
	MeterID    string         `json:"meter_id"`
	Resolution string         `json:"resolution"`
	Points     []ReadingPoint `json:"points"`
	Baseline   *BaselineInfo  `json:"baseline"`
	Thresholds Thresholds     `json:"thresholds"`
}

func (s *MeterService) Readings(ctx context.Context, meterID string, p ReadingsParams) (ReadingSeries, error) {
	p.Resolution = strings.ToLower(p.Resolution)
	if p.Resolution == "" {
		p.Resolution = "hour"
	}
	if !oneOf(p.Resolution, "hour", "day") {
		return ReadingSeries{}, fmt.Errorf("%w: resolution must be hour or day", ErrInvalidInput)
	}
	if p.From != nil && p.To != nil && p.To.Before(*p.From) {
		return ReadingSeries{}, fmt.Errorf("%w: to must be after from", ErrInvalidInput)
	}
	if _, err := s.repo.GetMeter(ctx, meterID); err != nil {
		return ReadingSeries{}, err
	}
	all, err := s.repo.ListReadings(ctx, store.ReadingsQuery{MeterID: meterID})
	if err != nil {
		return ReadingSeries{}, err
	}
	series := ReadingSeries{
		MeterID: meterID, Resolution: p.Resolution, Points: []ReadingPoint{},
		Thresholds: Thresholds{
			ShiftPct:            s.cfg.ShiftThreshold * 100,
			NominalVoltage:      s.cfg.NominalVoltage,
			VoltageTolerancePct: s.cfg.VoltageTolerance * 100,
		},
	}
	st, b, _, err := analytics.Snapshot(meterID, all, s.cfg)
	hasBaseline := err == nil
	if hasBaseline {
		series.Baseline = &BaselineInfo{From: b.From, To: b.To, Days: b.Days, DailyKWh: st.BaselineDailyKWh}
	}
	thr := s.cfg.ShiftThreshold
	for _, r := range all {
		if (p.From != nil && r.Timestamp.Before(*p.From)) || (p.To != nil && r.Timestamp.After(*p.To)) {
			continue
		}
		pt := ReadingPoint{Timestamp: r.Timestamp, ConsumptionKWh: r.ConsumptionKWh, VoltageV: r.VoltageV,
			CurrentA: r.CurrentA, PowerFactor: r.PowerFactor}
		if hasBaseline {
			h := r.Timestamp.UTC().Hour()
			pt.ExpectedKWh = b.KWh[h].Median
			pt.ExpectedV, pt.ExpectedA, pt.ExpectedPF = b.Voltage[h].Median, b.Current[h].Median, b.PF[h].Median
		}
		pt.LowerKWh, pt.UpperKWh = pt.ExpectedKWh*(1-thr), pt.ExpectedKWh*(1+thr)
		series.Points = append(series.Points, pt)
	}
	if p.Resolution == "day" {
		series.Points = daily(series.Points, thr)
	}
	return series, nil
}

func daily(points []ReadingPoint, thr float64) []ReadingPoint {
	var out []ReadingPoint
	var n float64
	flush := func() {
		if n == 0 {
			return
		}
		last := &out[len(out)-1]
		last.VoltageV, last.CurrentA, last.PowerFactor = last.VoltageV/n, last.CurrentA/n, last.PowerFactor/n
		last.ExpectedV, last.ExpectedA, last.ExpectedPF = last.ExpectedV/n, last.ExpectedA/n, last.ExpectedPF/n
		last.LowerKWh, last.UpperKWh = last.ExpectedKWh*(1-thr), last.ExpectedKWh*(1+thr)
	}
	for _, p := range points {
		d := p.Timestamp.UTC().Truncate(24 * time.Hour)
		if len(out) == 0 || !out[len(out)-1].Timestamp.Equal(d) {
			flush()
			out = append(out, ReadingPoint{Timestamp: d})
			n = 0
		}
		last := &out[len(out)-1]
		last.ConsumptionKWh += p.ConsumptionKWh
		last.ExpectedKWh += p.ExpectedKWh
		last.VoltageV += p.VoltageV
		last.CurrentA += p.CurrentA
		last.PowerFactor += p.PowerFactor
		last.ExpectedV += p.ExpectedV
		last.ExpectedA += p.ExpectedA
		last.ExpectedPF += p.ExpectedPF
		n++
	}
	flush()
	if out == nil {
		return []ReadingPoint{}
	}
	return out
}
