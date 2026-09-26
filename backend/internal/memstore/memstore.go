package memstore

import (
	"context"
	"sort"
	"sync"
	"time"

	"energyai/internal/domain"
	"energyai/internal/store"
)

type Store struct {
	mu        sync.Mutex
	meters    []domain.Meter
	readings  []domain.Reading
	events    []domain.Event
	runs      []domain.AnalysisRun
	anomalies []domain.Anomaly
	users     map[string]domain.User
	nextID    int64
}

func New(meters []domain.Meter, readings []domain.Reading, events []domain.Event) *Store {
	s := &Store{users: map[string]domain.User{}}
	for i, m := range meters {
		m.ID = int64(i + 1)
		if m.Status == "" {
			m.Status = domain.MeterOK
		}
		s.meters = append(s.meters, m)
	}
	s.readings = append(s.readings, readings...)
	sort.SliceStable(s.readings, func(i, j int) bool {
		if s.readings[i].MeterID != s.readings[j].MeterID {
			return s.readings[i].MeterID < s.readings[j].MeterID
		}
		return s.readings[i].Timestamp.Before(s.readings[j].Timestamp)
	})
	for i, e := range events {
		e.ID = int64(i + 1)
		s.events = append(s.events, e)
	}
	return s
}

func (s *Store) ListMeters(context.Context) ([]domain.Meter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.Meter(nil), s.meters...), nil
}

func (s *Store) GetMeter(_ context.Context, id string) (domain.Meter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.meters {
		if m.MeterID == id {
			return m, nil
		}
	}
	return domain.Meter{}, store.ErrNotFound
}

func (s *Store) ListReadings(_ context.Context, q store.ReadingsQuery) ([]domain.Reading, error) {
	var out []domain.Reading
	for _, r := range s.readings {
		if (q.MeterID == "" || r.MeterID == q.MeterID) &&
			(q.From == nil || !r.Timestamp.Before(*q.From)) && (q.To == nil || !r.Timestamp.After(*q.To)) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) ListEvents(_ context.Context, meterID string) ([]domain.Event, error) {
	var out []domain.Event
	for _, e := range s.events {
		if meterID == "" || e.MeterID == meterID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *Store) CreateRun(_ context.Context, run domain.AnalysisRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = append(s.runs, run.Clone())
	return nil
}

func (s *Store) setRun(run domain.AnalysisRun) {
	for i := range s.runs {
		if s.runs[i].ID == run.ID {
			s.runs[i] = run.Clone()
		}
	}
}

func (s *Store) UpdateRunProgress(_ context.Context, run domain.AnalysisRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setRun(run)
	return nil
}

func (s *Store) FailRun(_ context.Context, run domain.AnalysisRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	run.Status, run.FinishedAt = domain.RunFailed, &now
	s.setRun(run)
	return nil
}

func (s *Store) CompleteRun(_ context.Context, run domain.AnalysisRun, anomalies []domain.Anomaly) ([]domain.Anomaly, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := map[string]domain.Anomaly{}
	if latest, ok := s.latest(true); ok {
		for _, a := range s.anomalies {
			if a.AnalysisID == latest.ID {
				prev[a.MeterID+"|"+string(a.Type)] = a
			}
		}
	}
	out := append([]domain.Anomaly(nil), anomalies...)
	byMeter := map[string][]domain.Anomaly{}
	for i := range out {
		s.nextID++
		a := &out[i]
		a.ID, a.AnalysisID, a.UpdatedAt = s.nextID, run.ID, time.Now().UTC()
		if a.Status == "" {
			a.Status = domain.StatusOpen
		}
		if p, ok := prev[a.MeterID+"|"+string(a.Type)]; ok {
			a.Status, a.Note = p.Status, p.Note
		}
		s.anomalies = append(s.anomalies, *a)
		byMeter[a.MeterID] = append(byMeter[a.MeterID], *a)
	}
	for i := range s.meters {
		s.meters[i].Status = domain.MeterStatusFor(byMeter[s.meters[i].MeterID])
	}
	now := time.Now().UTC()
	run.Status, run.CurrentStep, run.FinishedAt = domain.RunCompleted, "", &now
	s.setRun(run)
	return out, nil
}

func (s *Store) GetRun(_ context.Context, id string) (domain.AnalysisRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.runs {
		if r.ID == id {
			return r.Clone(), nil
		}
	}
	return domain.AnalysisRun{}, store.ErrNotFound
}

func (s *Store) latest(completedOnly bool) (domain.AnalysisRun, bool) {
	for i := len(s.runs) - 1; i >= 0; i-- {
		if !completedOnly || s.runs[i].Status == domain.RunCompleted {
			return s.runs[i].Clone(), true
		}
	}
	return domain.AnalysisRun{}, false
}

func (s *Store) LatestRun(_ context.Context, completedOnly bool) (domain.AnalysisRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.latest(completedOnly); ok {
		return r, nil
	}
	return domain.AnalysisRun{}, store.ErrNotFound
}

func (s *Store) ListAnomalies(_ context.Context, f store.AnomalyFilter) ([]domain.Anomaly, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Anomaly
	for _, a := range s.anomalies {
		if a.AnalysisID == f.AnalysisID && (f.MeterID == "" || a.MeterID == f.MeterID) &&
			(f.Type == "" || a.Type == f.Type) && (f.Severity == "" || a.Severity == f.Severity) &&
			(f.Status == "" || a.Status == f.Status) {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PriorityScore > out[j].PriorityScore })
	return out, nil
}

func (s *Store) GetAnomaly(_ context.Context, id int64) (domain.Anomaly, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.anomalies {
		if a.ID == id {
			return a, nil
		}
	}
	return domain.Anomaly{}, store.ErrNotFound
}

func (s *Store) UpdateAnomalyStatus(_ context.Context, id int64, status domain.AnomalyStatus, note *string) (domain.Anomaly, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.anomalies {
		a := &s.anomalies[i]
		if a.ID != id {
			continue
		}
		a.Status, a.UpdatedAt = status, time.Now().UTC()
		if note != nil {
			a.Note = *note
		}
		var same []domain.Anomaly
		for _, o := range s.anomalies {
			if o.AnalysisID == a.AnalysisID && o.MeterID == a.MeterID {
				same = append(same, o)
			}
		}
		for j := range s.meters {
			if s.meters[j].MeterID == a.MeterID {
				s.meters[j].Status = domain.MeterStatusFor(same)
			}
		}
		return *a, nil
	}
	return domain.Anomaly{}, store.ErrNotFound
}

func (s *Store) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[email]; ok {
		return u, nil
	}
	return domain.User{}, store.ErrNotFound
}

func (s *Store) UpsertUser(_ context.Context, email, name, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[email] = domain.User{ID: int64(len(s.users) + 1), Email: email, Name: name, PasswordHash: hash}
	return nil
}
