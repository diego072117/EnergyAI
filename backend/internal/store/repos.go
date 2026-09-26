package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"energyai/internal/domain"
)

type MeterMeta struct {
	MeterID  string `json:"meter_id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

func (s *Store) SeedIfEmpty(ctx context.Context, metas []MeterMeta, readings []domain.Reading, events []domain.Event) (bool, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM meters`).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		for _, m := range metas {
			if _, err := tx.Exec(ctx, `INSERT INTO meters (meter_id, name, location) VALUES ($1, $2, $3)`,
				m.MeterID, m.Name, m.Location); err != nil {
				return err
			}
		}
		rows := make([][]any, len(readings))
		for i, r := range readings {
			rows[i] = []any{r.MeterID, r.Timestamp, r.ConsumptionKWh, r.VoltageV, r.CurrentA, r.PowerFactor, r.Status}
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"readings"},
			[]string{"meter_id", "timestamp", "consumption_kwh", "voltage_v", "current_a", "power_factor", "status"},
			pgx.CopyFromRows(rows)); err != nil {
			return err
		}
		for _, e := range events {
			if _, err := tx.Exec(ctx, `INSERT INTO events (meter_id, timestamp, type, description) VALUES ($1, $2, $3, $4)`,
				e.MeterID, e.Timestamp, string(e.Type), e.Description); err != nil {
				return err
			}
		}
		return nil
	})
	return err == nil, err
}

func (s *Store) ListMeters(ctx context.Context) ([]domain.Meter, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, meter_id, name, location, status, created_at FROM meters ORDER BY meter_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMeter)
}

func (s *Store) GetMeter(ctx context.Context, meterID string) (domain.Meter, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, meter_id, name, location, status, created_at FROM meters WHERE meter_id = $1`, meterID)
	if err != nil {
		return domain.Meter{}, err
	}
	m, err := pgx.CollectExactlyOneRow(rows, scanMeter)
	return m, notFound(err)
}

func scanMeter(row pgx.CollectableRow) (domain.Meter, error) {
	var m domain.Meter
	var status string
	err := row.Scan(&m.ID, &m.MeterID, &m.Name, &m.Location, &status, &m.CreatedAt)
	m.Status = domain.MeterStatus(status)
	return m, err
}

type ReadingsQuery struct {
	MeterID string
	From    *time.Time
	To      *time.Time
}

func (s *Store) ListReadings(ctx context.Context, q ReadingsQuery) ([]domain.Reading, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT meter_id, timestamp, consumption_kwh, voltage_v, current_a, power_factor, status
		FROM readings
		WHERE ($1 = '' OR meter_id = $1)
		  AND ($2::timestamptz IS NULL OR timestamp >= $2)
		  AND ($3::timestamptz IS NULL OR timestamp <= $3)
		ORDER BY meter_id, timestamp`, q.MeterID, q.From, q.To)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Reading, error) {
		var r domain.Reading
		err := row.Scan(&r.MeterID, &r.Timestamp, &r.ConsumptionKWh, &r.VoltageV, &r.CurrentA, &r.PowerFactor, &r.Status)
		r.Timestamp = r.Timestamp.UTC()
		return r, err
	})
}

func (s *Store) ListEvents(ctx context.Context, meterID string) ([]domain.Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, meter_id, timestamp, type, description FROM events
		WHERE ($1 = '' OR meter_id = $1) ORDER BY timestamp`, meterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Event, error) {
		var e domain.Event
		var typ string
		err := row.Scan(&e.ID, &e.MeterID, &e.Timestamp, &typ, &e.Description)
		e.Type = domain.EventType(typ)
		e.Timestamp = e.Timestamp.UTC()
		return e, err
	})
}

func (s *Store) UpsertUser(ctx context.Context, email, name, hash string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name, password_hash = EXCLUDED.password_hash`,
		email, name, hash)
	return err
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := s.pool.QueryRow(ctx, `SELECT id, email, name, password_hash FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash)
	return u, notFound(err)
}
