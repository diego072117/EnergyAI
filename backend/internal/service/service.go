package service

import (
	"context"
	"errors"

	"energyai/internal/domain"
	"energyai/internal/store"
)

var (
	ErrNotFound           = store.ErrNotFound
	ErrInvalidInput       = errors.New("invalid input")
	ErrAnalysisRunning    = errors.New("an analysis is already running")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type Repository interface {
	ListMeters(ctx context.Context) ([]domain.Meter, error)
	GetMeter(ctx context.Context, meterID string) (domain.Meter, error)
	ListReadings(ctx context.Context, q store.ReadingsQuery) ([]domain.Reading, error)
	ListEvents(ctx context.Context, meterID string) ([]domain.Event, error)

	CreateRun(ctx context.Context, run domain.AnalysisRun) error
	UpdateRunProgress(ctx context.Context, run domain.AnalysisRun) error
	FailRun(ctx context.Context, run domain.AnalysisRun) error
	CompleteRun(ctx context.Context, run domain.AnalysisRun, anomalies []domain.Anomaly) ([]domain.Anomaly, error)
	GetRun(ctx context.Context, id string) (domain.AnalysisRun, error)
	LatestRun(ctx context.Context, completedOnly bool) (domain.AnalysisRun, error)

	ListAnomalies(ctx context.Context, f store.AnomalyFilter) ([]domain.Anomaly, error)
	GetAnomaly(ctx context.Context, id int64) (domain.Anomaly, error)
	UpdateAnomalyStatus(ctx context.Context, id int64, status domain.AnomalyStatus, note *string) (domain.Anomaly, error)

	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	UpsertUser(ctx context.Context, email, name, hash string) error
}

func latestAnomalies(ctx context.Context, repo Repository, f store.AnomalyFilter) (string, []domain.Anomaly, error) {
	run, err := repo.LatestRun(ctx, true)
	if errors.Is(err, store.ErrNotFound) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	f.AnalysisID = run.ID
	as, err := repo.ListAnomalies(ctx, f)
	return run.ID, as, err
}
