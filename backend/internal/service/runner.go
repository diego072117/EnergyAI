package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"energyai/internal/ai"
	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/store"
)

type Runner struct {
	repo        Repository
	explainer   ai.Explainer
	cfg         analytics.Config
	stepDelay   time.Duration
	concurrency int
	log         *slog.Logger

	mu      sync.Mutex
	running bool
	wg      sync.WaitGroup
	baseCtx context.Context
	cancel  context.CancelFunc
}

type RunnerOptions struct {
	// StepDelay is the minimum visible duration of each step. The engine
	// takes milliseconds; a small delay lets people follow the progress.
	StepDelay   time.Duration
	Concurrency int
}

func NewRunner(repo Repository, explainer ai.Explainer, cfg analytics.Config, opts RunnerOptions, log *slog.Logger) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	return &Runner{repo: repo, explainer: explainer, cfg: cfg, stepDelay: opts.StepDelay,
		concurrency: opts.Concurrency, log: log, baseCtx: ctx, cancel: cancel}
}

func (r *Runner) Start(ctx context.Context) (domain.AnalysisRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return domain.AnalysisRun{}, ErrAnalysisRunning
	}
	run := domain.AnalysisRun{
		ID:        newID(),
		Status:    domain.RunPending,
		Steps:     domain.NewAnalysisSteps(),
		StartedAt: time.Now().UTC(),
	}
	if err := r.repo.CreateRun(ctx, run); err != nil {
		return domain.AnalysisRun{}, err
	}
	r.running = true
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer func() {
			r.mu.Lock()
			r.running = false
			r.mu.Unlock()
		}()
		r.execute(run)
	}()
	return run, nil
}

func (r *Runner) Get(ctx context.Context, id string) (domain.AnalysisRun, error) {
	return r.repo.GetRun(ctx, id)
}

func (r *Runner) Latest(ctx context.Context) (domain.AnalysisRun, error) {
	return r.repo.LatestRun(ctx, false)
}

func (r *Runner) Shutdown() {
	r.cancel()
	r.wg.Wait()
}

func (r *Runner) Wait() { r.wg.Wait() }

type stepFunc func(ctx context.Context) (string, error)

func (r *Runner) execute(run domain.AnalysisRun) {
	ctx, cancel := context.WithTimeout(r.baseCtx, 10*time.Minute)
	defer cancel()
	run.Status = domain.RunRunning

	var (
		pipeline     *analytics.Pipeline
		readingCount int
		findings     []analytics.Finding
		explanations []ai.Explanation
		anomalies    []domain.Anomaly
	)
	steps := []stepFunc{
		func(ctx context.Context) (string, error) { // READINGS
			readings, err := r.repo.ListReadings(ctx, store.ReadingsQuery{})
			if err != nil {
				return "", err
			}
			events, err := r.repo.ListEvents(ctx, "")
			if err != nil {
				return "", err
			}
			readingCount = len(readings)
			pipeline = analytics.NewPipeline(r.cfg, analytics.BuildInputs(readings, events))
			return pipeline.Validate(), nil
		},
		func(context.Context) (string, error) { return pipeline.ComputeBaselines(), nil }, // BASELINE
		func(context.Context) (string, error) { return pipeline.Detect(), nil },           // DETECTION
		func(context.Context) (string, error) { return pipeline.Correlate(), nil },        // CORRELATION
		func(context.Context) (string, error) { // EVENTS
			detail := pipeline.MatchEvents()
			findings = pipeline.Findings()
			return detail, nil
		},
		func(ctx context.Context) (string, error) { // EXPLANATION
			var err error
			explanations, err = r.explainAll(ctx, findings)
			if err != nil {
				return "", err
			}
			return explanationDetail(explanations), nil
		},
		func(ctx context.Context) (string, error) { // RECOMMENDATION
			anomalies = buildAnomalies(findings, explanations)
			run.Summary = summarizeRun(len(pipeline.Stats()), readingCount, anomalies)
			return fmt.Sprintf("%d acciones recomendadas y priorizadas", len(anomalies)), nil
		},
	}

	for i, step := range steps {
		if err := r.runStep(ctx, &run, i, step); err != nil {
			r.log.Error("analysis failed", "run", run.ID, "step", run.Steps[i].Name, "err", err)
			run.Error = err.Error()
			if ferr := r.repo.FailRun(context.WithoutCancel(ctx), run); ferr != nil {
				r.log.Error("persisting failed run", "run", run.ID, "err", ferr)
			}
			return
		}
	}
	if _, err := r.repo.CompleteRun(context.WithoutCancel(ctx), run, anomalies); err != nil {
		r.log.Error("persisting analysis", "run", run.ID, "err", err)
		run.Error = err.Error()
		_ = r.repo.FailRun(context.WithoutCancel(ctx), run)
		return
	}
	r.log.Info("analysis completed", "run", run.ID, "findings", len(findings))
}

func (r *Runner) runStep(ctx context.Context, run *domain.AnalysisRun, i int, fn stepFunc) error {
	start := time.Now().UTC()
	st := &run.Steps[i]
	st.State, st.StartedAt = domain.StepRunning, &start
	run.CurrentStep = st.Name
	if err := r.repo.UpdateRunProgress(ctx, *run); err != nil {
		return err
	}
	detail, err := fn(ctx)
	if err == nil {
		err = sleepCtx(ctx, r.stepDelay-time.Since(start))
	}
	end := time.Now().UTC()
	st.EndedAt = &end
	if err != nil {
		st.State, st.Detail = domain.StepFailed, err.Error()
		return err
	}
	st.State, st.Detail = domain.StepDone, detail
	return r.repo.UpdateRunProgress(ctx, *run)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (r *Runner) explainAll(ctx context.Context, fs []analytics.Finding) ([]ai.Explanation, error) {
	out := make([]ai.Explanation, len(fs))
	errs := make([]error, len(fs))
	sem := make(chan struct{}, r.concurrency)
	var wg sync.WaitGroup
	for i, f := range fs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i], errs[i] = r.explainer.Explain(ctx, f)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("explaining %s: %w", fs[i].MeterID, err)
		}
	}
	return out, ctx.Err()
}

func explanationDetail(exps []ai.Explanation) string {
	llm, tpl, model := 0, 0, ""
	for _, e := range exps {
		if strings.HasPrefix(e.Source, "LLM:") {
			llm++
			model = strings.TrimPrefix(e.Source, "LLM:")
		} else {
			tpl++
		}
	}
	switch {
	case llm > 0 && tpl > 0:
		return fmt.Sprintf("%d explicaciones con IA generativa (%s) y %d con reglas", llm, model, tpl)
	case llm > 0:
		return fmt.Sprintf("%d explicaciones redactadas con IA generativa (%s)", llm, model)
	}
	return fmt.Sprintf("%d explicaciones generadas a partir de la evidencia", tpl)
}

func buildAnomalies(fs []analytics.Finding, exps []ai.Explanation) []domain.Anomaly {
	out := make([]domain.Anomaly, 0, len(fs))
	for i, f := range fs {
		e := exps[i]
		out = append(out, domain.Anomaly{
			MeterID:           f.MeterID,
			DetectedAt:        f.WindowStart,
			Type:              f.Type,
			IsAnomaly:         f.IsAnomaly(),
			Severity:          f.Severity,
			Confidence:        f.Confidence,
			PriorityScore:     f.PriorityScore,
			Reason:            e.Reason,
			RecommendedAction: e.RecommendedAction,
			EvidenceSummary:   e.EvidenceSummary,
			Status:            domain.StatusOpen,
			WindowStart:       f.WindowStart,
			WindowEnd:         f.WindowEnd,
			Evidence:          f.Evidence,
			ExplanationSource: e.Source,
		})
	}
	return out
}

func summarizeRun(meters, readings int, as []domain.Anomaly) *domain.AnalysisSummary {
	s := &domain.AnalysisSummary{MetersAnalyzed: meters, ReadingsAnalyzed: readings, Detected: len(as), ByType: map[string]int{}}
	sources := map[string]bool{}
	conf := 0.0
	for _, a := range as {
		s.ByType[string(a.Type)]++
		conf += a.Confidence
		sources[a.ExplanationSource] = true
		if a.IsAnomaly && a.Severity == domain.SeverityHigh {
			s.HighPriority++
		}
	}
	if len(as) > 0 {
		s.AvgConfidence = roundTo(conf/float64(len(as)), 2)
	}
	var src []string
	for k := range sources {
		src = append(src, k)
	}
	s.ExplanationSource = strings.Join(src, ", ")
	s.Text = fmt.Sprintf("%d %s · %d %s atención prioritaria",
		s.Detected, plural(s.Detected, "anomalía detectada", "anomalías detectadas"),
		s.HighPriority, plural(s.HighPriority, "requiere", "requieren"))
	if s.Detected == 0 {
		s.Text = "Sin anomalías: todos los medidores operan dentro de su comportamiento esperado"
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
