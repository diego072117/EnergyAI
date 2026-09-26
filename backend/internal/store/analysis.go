package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"energyai/internal/domain"
)

func (s *Store) CreateRun(ctx context.Context, run domain.AnalysisRun) error {
	steps, err := json.Marshal(run.Steps)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO analysis_runs (id, status, current_step, steps, started_at) VALUES ($1, $2, $3, $4, $5)`,
		run.ID, string(run.Status), run.CurrentStep, steps, run.StartedAt)
	return err
}

func (s *Store) UpdateRunProgress(ctx context.Context, run domain.AnalysisRun) error {
	steps, err := json.Marshal(run.Steps)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE analysis_runs SET status = $2, current_step = $3, steps = $4 WHERE id = $1`,
		run.ID, string(run.Status), run.CurrentStep, steps)
	return err
}

func (s *Store) FailRun(ctx context.Context, run domain.AnalysisRun) error {
	steps, err := json.Marshal(run.Steps)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE analysis_runs SET status = $2, steps = $3, error = $4, finished_at = $5 WHERE id = $1`,
		run.ID, string(domain.RunFailed), steps, run.Error, time.Now().UTC())
	return err
}

func (s *Store) FailInterruptedRuns(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE analysis_runs SET status = 'FAILED', error = 'interrumpido por reinicio del servidor', finished_at = now()
		WHERE status IN ('PENDING', 'RUNNING')`)
	return tag.RowsAffected(), err
}

// CompleteRun stores the findings of a run atomically: it carries over the
// workflow status that operators set on equivalent findings of the previous
// run, inserts the anomalies, updates meter statuses and closes the run.
func (s *Store) CompleteRun(ctx context.Context, run domain.AnalysisRun, anomalies []domain.Anomaly) ([]domain.Anomaly, error) {
	steps, err := json.Marshal(run.Steps)
	if err != nil {
		return nil, err
	}
	summary, err := json.Marshal(run.Summary)
	if err != nil {
		return nil, err
	}
	out := append([]domain.Anomaly(nil), anomalies...)
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		prev, err := previousStatuses(ctx, tx)
		if err != nil {
			return err
		}
		byMeter := map[string][]domain.Anomaly{}
		for i := range out {
			a := &out[i]
			a.AnalysisID = run.ID
			if a.Status == "" {
				a.Status = domain.StatusOpen
			}
			if p, ok := prev[a.MeterID+"|"+string(a.Type)]; ok {
				a.Status, a.Note = p.status, p.note
			}
			ev, err := json.Marshal(a.Evidence)
			if err != nil {
				return err
			}
			summ, err := json.Marshal(nonNil(a.EvidenceSummary))
			if err != nil {
				return err
			}
			err = tx.QueryRow(ctx, `
				INSERT INTO anomalies (analysis_id, meter_id, detected_at, type, is_anomaly, severity, confidence,
					priority_score, reason, recommended_action, evidence_summary, status, note, window_start,
					window_end, evidence, explanation_source)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
				RETURNING id, updated_at`,
				a.AnalysisID, a.MeterID, a.DetectedAt, string(a.Type), a.IsAnomaly, string(a.Severity), a.Confidence,
				a.PriorityScore, a.Reason, a.RecommendedAction, summ, string(a.Status), a.Note, a.WindowStart,
				a.WindowEnd, ev, a.ExplanationSource).Scan(&a.ID, &a.UpdatedAt)
			if err != nil {
				return err
			}
			byMeter[a.MeterID] = append(byMeter[a.MeterID], *a)
		}
		if _, err := tx.Exec(ctx, `UPDATE meters SET status = 'OK'`); err != nil {
			return err
		}
		for meter, as := range byMeter {
			if _, err := tx.Exec(ctx, `UPDATE meters SET status = $2 WHERE meter_id = $1`, meter, string(domain.MeterStatusFor(as))); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE analysis_runs SET status = $2, current_step = '', steps = $3, summary = $4, finished_at = $5 WHERE id = $1`,
			run.ID, string(domain.RunCompleted), steps, summary, time.Now().UTC())
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

type prevStatus struct {
	status domain.AnomalyStatus
	note   string
}

func previousStatuses(ctx context.Context, tx pgx.Tx) (map[string]prevStatus, error) {
	rows, err := tx.Query(ctx, `
		SELECT a.meter_id, a.type, a.status, a.note FROM anomalies a
		WHERE a.analysis_id = (SELECT id FROM analysis_runs WHERE status = 'COMPLETED' ORDER BY started_at DESC LIMIT 1)
		  AND (a.status <> 'OPEN' OR a.note <> '')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]prevStatus{}
	for rows.Next() {
		var meter, typ, status, note string
		if err := rows.Scan(&meter, &typ, &status, &note); err != nil {
			return nil, err
		}
		out[meter+"|"+typ] = prevStatus{domain.AnomalyStatus(status), note}
	}
	return out, rows.Err()
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

const runColumns = `id, status, current_step, steps, summary, error, started_at, finished_at`

func scanRun(row pgx.CollectableRow) (domain.AnalysisRun, error) {
	var r domain.AnalysisRun
	var status string
	var steps, summary []byte
	if err := row.Scan(&r.ID, &status, &r.CurrentStep, &steps, &summary, &r.Error, &r.StartedAt, &r.FinishedAt); err != nil {
		return r, err
	}
	r.Status = domain.RunStatus(status)
	if err := json.Unmarshal(steps, &r.Steps); err != nil {
		return r, fmt.Errorf("decoding steps: %w", err)
	}
	if len(summary) > 0 && string(summary) != "null" {
		r.Summary = &domain.AnalysisSummary{}
		if err := json.Unmarshal(summary, r.Summary); err != nil {
			return r, fmt.Errorf("decoding summary: %w", err)
		}
	}
	return r, nil
}

func (s *Store) GetRun(ctx context.Context, id string) (domain.AnalysisRun, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+runColumns+` FROM analysis_runs WHERE id = $1`, id)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	r, err := pgx.CollectExactlyOneRow(rows, scanRun)
	return r, notFound(err)
}

func (s *Store) LatestRun(ctx context.Context, completedOnly bool) (domain.AnalysisRun, error) {
	q := `SELECT ` + runColumns + ` FROM analysis_runs`
	if completedOnly {
		q += ` WHERE status = 'COMPLETED'`
	}
	rows, err := s.pool.Query(ctx, q+` ORDER BY started_at DESC LIMIT 1`)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	r, err := pgx.CollectExactlyOneRow(rows, scanRun)
	return r, notFound(err)
}

type AnomalyFilter struct {
	AnalysisID string
	MeterID    string
	Type       domain.AnomalyType
	Severity   domain.Severity
	Status     domain.AnomalyStatus
}

const anomalyColumns = `id, analysis_id, meter_id, detected_at, type, is_anomaly, severity, confidence, priority_score,
	reason, recommended_action, evidence_summary, status, note, window_start, window_end, evidence, explanation_source, updated_at`

func scanAnomaly(row pgx.CollectableRow) (domain.Anomaly, error) {
	var a domain.Anomaly
	var typ, sev, status string
	var summary, evidence []byte
	err := row.Scan(&a.ID, &a.AnalysisID, &a.MeterID, &a.DetectedAt, &typ, &a.IsAnomaly, &sev, &a.Confidence,
		&a.PriorityScore, &a.Reason, &a.RecommendedAction, &summary, &status, &a.Note, &a.WindowStart,
		&a.WindowEnd, &evidence, &a.ExplanationSource, &a.UpdatedAt)
	if err != nil {
		return a, err
	}
	a.Type, a.Severity, a.Status = domain.AnomalyType(typ), domain.Severity(sev), domain.AnomalyStatus(status)
	a.DetectedAt, a.WindowStart, a.WindowEnd = a.DetectedAt.UTC(), a.WindowStart.UTC(), a.WindowEnd.UTC()
	if err := json.Unmarshal(summary, &a.EvidenceSummary); err != nil {
		return a, fmt.Errorf("decoding evidence_summary: %w", err)
	}
	if err := json.Unmarshal(evidence, &a.Evidence); err != nil {
		return a, fmt.Errorf("decoding evidence: %w", err)
	}
	return a, nil
}

func (s *Store) ListAnomalies(ctx context.Context, f AnomalyFilter) ([]domain.Anomaly, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	add("analysis_id = $%d", f.AnalysisID)
	if f.MeterID != "" {
		add("meter_id = $%d", f.MeterID)
	}
	if f.Type != "" {
		add("type = $%d", string(f.Type))
	}
	if f.Severity != "" {
		add("severity = $%d", string(f.Severity))
	}
	if f.Status != "" {
		add("status = $%d", string(f.Status))
	}
	rows, err := s.pool.Query(ctx, `SELECT `+anomalyColumns+` FROM anomalies WHERE `+strings.Join(where, " AND ")+
		` ORDER BY priority_score DESC, confidence DESC, meter_id`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanAnomaly)
}

func (s *Store) GetAnomaly(ctx context.Context, id int64) (domain.Anomaly, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+anomalyColumns+` FROM anomalies WHERE id = $1`, id)
	if err != nil {
		return domain.Anomaly{}, err
	}
	a, err := pgx.CollectExactlyOneRow(rows, scanAnomaly)
	return a, notFound(err)
}

func (s *Store) UpdateAnomalyStatus(ctx context.Context, id int64, status domain.AnomalyStatus, note *string) (domain.Anomaly, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var meter, analysis string
		err := tx.QueryRow(ctx, `
			UPDATE anomalies SET status = $2, note = COALESCE($3, note), updated_at = now()
			WHERE id = $1 RETURNING meter_id, analysis_id`, id, string(status), note).Scan(&meter, &analysis)
		if err != nil {
			return notFound(err)
		}
		// Only the latest completed run drives the meter status.
		var latest string
		if err := tx.QueryRow(ctx, `SELECT id FROM analysis_runs WHERE status = 'COMPLETED' ORDER BY started_at DESC LIMIT 1`).Scan(&latest); err != nil {
			return err
		}
		if latest != analysis {
			return nil
		}
		rows, err := tx.Query(ctx, `SELECT `+anomalyColumns+` FROM anomalies WHERE analysis_id = $1 AND meter_id = $2`, analysis, meter)
		if err != nil {
			return err
		}
		as, err := pgx.CollectRows(rows, scanAnomaly)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE meters SET status = $2 WHERE meter_id = $1`, meter, string(domain.MeterStatusFor(as)))
		return err
	})
	if err != nil {
		return domain.Anomaly{}, err
	}
	return s.GetAnomaly(ctx, id)
}
