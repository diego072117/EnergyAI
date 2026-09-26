package ai

import (
	"context"
	"log/slog"

	"energyai/internal/analytics"
)

type Explanation struct {
	Reason            string   `json:"reason"`
	RecommendedAction string   `json:"recommended_action"`
	EvidenceSummary   []string `json:"evidence_summary"`
	Source            string   `json:"source"` // "TEMPLATE" or "LLM:<model>"
}

type Explainer interface {
	Explain(ctx context.Context, f analytics.Finding) (Explanation, error)
}

type Status struct {
	Provider  string `json:"provider"`
	Model     string `json:"model,omitempty"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

type StatusReporter interface {
	Status(ctx context.Context) Status
}

type WithFallback struct {
	Primary  Explainer
	Fallback Explainer
	Log      *slog.Logger
}

func (w WithFallback) Explain(ctx context.Context, f analytics.Finding) (Explanation, error) {
	exp, err := w.Primary.Explain(ctx, f)
	if err == nil {
		return exp, nil
	}
	if w.Log != nil {
		w.Log.Warn("LLM explanation failed, using template", "meter", f.MeterID, "err", err)
	}
	return w.Fallback.Explain(ctx, f)
}

func (w WithFallback) Status(ctx context.Context) Status {
	if sr, ok := w.Primary.(StatusReporter); ok {
		return sr.Status(ctx)
	}
	return Status{Provider: "template", Available: true}
}
