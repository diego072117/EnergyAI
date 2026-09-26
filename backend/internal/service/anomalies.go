package service

import (
	"context"
	"fmt"
	"strings"

	"energyai/internal/domain"
	"energyai/internal/store"
)

type AnomalyService struct {
	repo Repository
}

func NewAnomalyService(repo Repository) *AnomalyService { return &AnomalyService{repo: repo} }

type AnomalyQuery struct {
	AnalysisID string
	MeterID    string
	Type       string
	Severity   string
	Status     string
}

type AnomalyList struct {
	AnalysisID string           `json:"analysis_id"`
	Items      []domain.Anomaly `json:"items"`
	Total      int              `json:"total"`
}

func (s *AnomalyService) List(ctx context.Context, q AnomalyQuery) (AnomalyList, error) {
	f := store.AnomalyFilter{
		MeterID:  q.MeterID,
		Type:     domain.AnomalyType(strings.ToUpper(q.Type)),
		Severity: domain.Severity(strings.ToUpper(q.Severity)),
		Status:   domain.AnomalyStatus(strings.ToUpper(q.Status)),
	}
	if f.Type != "" && !f.Type.Valid() {
		return AnomalyList{}, fmt.Errorf("%w: unknown type %q", ErrInvalidInput, q.Type)
	}
	if f.Severity != "" && !f.Severity.Valid() {
		return AnomalyList{}, fmt.Errorf("%w: unknown severity %q", ErrInvalidInput, q.Severity)
	}
	if f.Status != "" && !f.Status.Valid() {
		return AnomalyList{}, fmt.Errorf("%w: unknown status %q", ErrInvalidInput, q.Status)
	}
	var (
		id  string
		as  []domain.Anomaly
		err error
	)
	if q.AnalysisID != "" {
		if _, err = s.repo.GetRun(ctx, q.AnalysisID); err != nil {
			return AnomalyList{}, err
		}
		f.AnalysisID, id = q.AnalysisID, q.AnalysisID
		as, err = s.repo.ListAnomalies(ctx, f)
	} else {
		id, as, err = latestAnomalies(ctx, s.repo, f)
	}
	if err != nil {
		return AnomalyList{}, err
	}
	as = nonNilAnomalies(as)
	return AnomalyList{AnalysisID: id, Items: as, Total: len(as)}, nil
}

type AnomalyDetail struct {
	domain.Anomaly
	Meter domain.Meter `json:"meter"`
	Rank  int          `json:"rank"` // position in the priority list of its run
	Of    int          `json:"of"`
}

func (s *AnomalyService) Get(ctx context.Context, id int64) (AnomalyDetail, error) {
	a, err := s.repo.GetAnomaly(ctx, id)
	if err != nil {
		return AnomalyDetail{}, err
	}
	m, err := s.repo.GetMeter(ctx, a.MeterID)
	if err != nil {
		return AnomalyDetail{}, err
	}
	all, err := s.repo.ListAnomalies(ctx, store.AnomalyFilter{AnalysisID: a.AnalysisID})
	if err != nil {
		return AnomalyDetail{}, err
	}
	d := AnomalyDetail{Anomaly: a, Meter: m, Of: len(all)}
	for i, o := range all {
		if o.ID == a.ID {
			d.Rank = i + 1
		}
	}
	return d, nil
}

func (s *AnomalyService) UpdateStatus(ctx context.Context, id int64, status string, note *string) (domain.Anomaly, error) {
	st := domain.AnomalyStatus(strings.ToUpper(status))
	if !st.Valid() {
		return domain.Anomaly{}, fmt.Errorf("%w: status must be OPEN, INVESTIGATING, RESOLVED or DISMISSED", ErrInvalidInput)
	}
	if note != nil {
		n := strings.TrimSpace(*note)
		if len(n) > 1000 {
			return domain.Anomaly{}, fmt.Errorf("%w: note is too long", ErrInvalidInput)
		}
		note = &n
	}
	return s.repo.UpdateAnomalyStatus(ctx, id, st, note)
}
