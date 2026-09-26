package service

import (
	"context"
	"errors"
	"math"
	"time"

	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/store"
)

type DashboardService struct {
	repo Repository
	cfg  analytics.Config
}

func NewDashboardService(repo Repository, cfg analytics.Config) *DashboardService {
	return &DashboardService{repo: repo, cfg: cfg}
}

type AnomalyCounts struct {
	Detected     int            `json:"detected"`
	HighPriority int            `json:"high_priority"`
	Active       int            `json:"active"`
	ByType       map[string]int `json:"by_type"`
}

type DailyPoint struct {
	Date        string  `json:"date"`
	KWh         float64 `json:"kwh"`
	BaselineKWh float64 `json:"baseline_kwh"`
}

type DashboardSummary struct {
	Meters              int                 `json:"meters"`
	MetersByStatus      map[string]int      `json:"meters_by_status"`
	TotalConsumptionKWh float64             `json:"total_consumption_kwh"`
	PeriodStart         *time.Time          `json:"period_start"`
	PeriodEnd           *time.Time          `json:"period_end"`
	Anomalies           AnomalyCounts       `json:"anomalies"`
	AvgConfidence       float64             `json:"avg_confidence"`
	LastAnalysis        *domain.AnalysisRun `json:"last_analysis"`
	TopPriority         []domain.Anomaly    `json:"top_priority"`
	DailyConsumption    []DailyPoint        `json:"daily_consumption"`
}

func (s *DashboardService) Summary(ctx context.Context) (DashboardSummary, error) {
	meters, err := s.repo.ListMeters(ctx)
	if err != nil {
		return DashboardSummary{}, err
	}
	readings, err := s.repo.ListReadings(ctx, store.ReadingsQuery{})
	if err != nil {
		return DashboardSummary{}, err
	}
	sum := DashboardSummary{
		Meters:           len(meters),
		MetersByStatus:   map[string]int{string(domain.MeterOK): 0, string(domain.MeterAlert): 0, string(domain.MeterCritical): 0},
		Anomalies:        AnomalyCounts{ByType: map[string]int{}},
		TopPriority:      []domain.Anomaly{},
		DailyConsumption: []DailyPoint{},
	}
	for _, m := range meters {
		sum.MetersByStatus[string(m.Status)]++
	}

	baseline := 0.0
	for _, in := range analytics.BuildInputs(readings, nil) {
		if st, _, _, err := analytics.Snapshot(in.MeterID, in.Readings, s.cfg); err == nil {
			baseline += st.BaselineDailyKWh
		}
	}
	idx := map[string]int{}
	for _, r := range readings {
		sum.TotalConsumptionKWh += r.ConsumptionKWh
		ts := r.Timestamp
		if sum.PeriodStart == nil || ts.Before(*sum.PeriodStart) {
			sum.PeriodStart = &ts
		}
		if sum.PeriodEnd == nil || ts.After(*sum.PeriodEnd) {
			sum.PeriodEnd = &ts
		}
		d := ts.UTC().Format("2006-01-02")
		i, ok := idx[d]
		if !ok {
			i = len(sum.DailyConsumption)
			idx[d] = i
			sum.DailyConsumption = append(sum.DailyConsumption, DailyPoint{Date: d, BaselineKWh: roundTo(baseline, 1)})
		}
		sum.DailyConsumption[i].KWh += r.ConsumptionKWh
	}
	for i := range sum.DailyConsumption {
		sum.DailyConsumption[i].KWh = roundTo(sum.DailyConsumption[i].KWh, 1)
	}
	sum.TotalConsumptionKWh = roundTo(sum.TotalConsumptionKWh, 1)

	if run, err := s.repo.LatestRun(ctx, false); err == nil {
		sum.LastAnalysis = &run
	} else if !errors.Is(err, ErrNotFound) {
		return DashboardSummary{}, err
	}
	_, as, err := latestAnomalies(ctx, s.repo, store.AnomalyFilter{})
	if err != nil {
		return DashboardSummary{}, err
	}
	conf := 0.0
	for _, a := range as {
		sum.Anomalies.Detected++
		sum.Anomalies.ByType[string(a.Type)]++
		conf += a.Confidence
		if a.IsAnomaly && a.Severity == domain.SeverityHigh {
			sum.Anomalies.HighPriority++
		}
		if a.IsAnomaly && a.Status.Active() {
			sum.Anomalies.Active++
			if a.Severity == domain.SeverityHigh && len(sum.TopPriority) < 5 {
				sum.TopPriority = append(sum.TopPriority, a)
			}
		}
	}
	if len(as) > 0 {
		sum.AvgConfidence = roundTo(conf/float64(len(as)), 2)
	}
	return sum, nil
}

func roundTo(x float64, n int) float64 {
	p := math.Pow(10, float64(n))
	return math.Round(x*p) / p
}
