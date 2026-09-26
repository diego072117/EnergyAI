package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/format"
)

const (
	ProviderOllama = "ollama"
	ProviderOpenAI = "openai" // any OpenAI-compatible API (OpenAI, Groq, OpenRouter, Gemini...)
)

type LLMConfig struct {
	Provider string
	BaseURL  string
	Model    string
	APIKey   string
	Timeout  time.Duration
}

// LLM writes explanations with a language model. It only receives the
// computed evidence and the rules-based draft, never raw data, and its output
// is validated before being accepted. The model writes the reason and the
// action; the evidence list stays deterministic (computed by the engine).
type LLM struct {
	cfg    LLMConfig
	client *http.Client
}

func NewLLM(cfg LLMConfig) *LLM {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Timeout <= 0 {
		cfg.Timeout = 90 * time.Second
	}
	return &LLM{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (l *LLM) Source() string { return "LLM:" + l.cfg.Model }

const systemPrompt = `Eres un analista senior de gestión energética industrial. Redactas explicaciones breves, claras y accionables en español para operadores de planta.
Reglas estrictas:
- Usa EXCLUSIVAMENTE los datos del JSON de evidencia. No inventes cifras, fechas, equipos ni causas que la evidencia no soporte.
- Si mencionas un número, cópialo de la evidencia (puedes redondearlo).
- No cambies el tipo ni la severidad: ya fueron decididos por el motor de análisis.
- "reason": máximo 3 frases; explica qué pasó, desde cuándo y por qué se clasifica así.
- "recommended_action": 1 o 2 frases en imperativo, coherentes con la severidad.
Responde SOLO con un objeto JSON con las claves reason y recommended_action.`

var outputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"reason":             map[string]any{"type": "string"},
		"recommended_action": map[string]any{"type": "string"},
	},
	"required": []string{"reason", "recommended_action"},
}

var typeLabels = map[domain.AnomalyType]string{
	domain.AnomalyReal:          "Anomalía real (sin explicación operativa)",
	domain.AnomalyExplainable:   "Anomalía explicable por un evento operativo",
	domain.AnomalyDataQuality:   "Problema de calidad de datos del medidor",
	domain.AnomalyFalsePositive: "Falso positivo explicado por un evento operativo",
}

func (l *LLM) Explain(ctx context.Context, f analytics.Finding) (Explanation, error) {
	draft, err := Template{}.Explain(ctx, f)
	if err != nil {
		return Explanation{}, err
	}
	user, err := buildPrompt(f, draft)
	if err != nil {
		return Explanation{}, err
	}
	content, err := l.chat(ctx, user)
	if err != nil {
		return Explanation{}, err
	}
	var out Explanation
	if err := json.Unmarshal([]byte(extractJSON(content)), &out); err != nil {
		return Explanation{}, fmt.Errorf("invalid JSON from model: %w", err)
	}
	if err := Validate(out, f); err != nil {
		return Explanation{}, fmt.Errorf("model output rejected: %w", err)
	}
	out.EvidenceSummary = draft.EvidenceSummary
	out.Source = l.Source()
	return out, nil
}

func buildPrompt(f analytics.Finding, draft Explanation) (string, error) {
	ev := f.Evidence
	var events []map[string]any
	for _, e := range ev.Events {
		events = append(events, map[string]any{
			"tipo": e.Type, "fecha": format.Date(e.Timestamp), "descripcion": e.Description,
			"explica_el_cambio": e.Explains, "nota": e.Note,
		})
	}
	var vars []map[string]any
	for _, v := range ev.Variables {
		vars = append(vars, map[string]any{
			"variable": v.Label, "unidad": v.Unit, "esperado_baseline": v.Before, "observado": v.After,
			"min": v.Min, "max": v.Max, "cambio_pct": v.ChangePct, "relevante": v.Significant,
		})
	}
	payload := map[string]any{
		"medidor":   f.MeterID,
		"tipo":      typeLabels[f.Type],
		"severidad": f.Severity,
		"confianza": fmt.Sprintf("%.0f%%", f.Confidence*100),
		"evidencia": map[string]any{
			"desviacion_consumo_pct": ev.DeviationPct,
			"ventana":                map[string]any{"inicio": format.Date(ev.Window.Start), "fin": format.Date(ev.Window.End), "horas": ev.Window.Hours, "sigue_activo": ev.Window.Persistent},
			"baseline_diario_kwh":    ev.Baseline.DailyKWh,
			"consumo_ultimas_24h":    ev.Current.Last24hKWh,
			"variacion_24h_pct":      ev.Current.VariationPct,
			"variables":              vars,
			"senales_que_soportan":   supportingSignals(ev),
			"eventos":                events,
			"calidad_de_datos":       ev.DataQuality,
		},
		"borrador_del_motor_de_reglas": map[string]string{"reason": draft.Reason, "recommended_action": draft.RecommendedAction},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "Redacta la explicación de este hallazgo. Puedes mejorar la redacción del borrador, pero no agregar información que no esté en la evidencia.\n\n" + string(b), nil
}

func (l *LLM) chat(ctx context.Context, user string) (string, error) {
	messages := []map[string]string{{"role": "system", "content": systemPrompt}, {"role": "user", "content": user}}
	switch l.cfg.Provider {
	case ProviderOllama:
		var resp struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		body := map[string]any{
			"model": l.cfg.Model, "stream": false, "format": outputSchema, "keep_alive": "15m",
			"options": map[string]any{"temperature": 0.2}, "messages": messages,
		}
		if err := l.post(ctx, l.cfg.BaseURL+"/api/chat", body, &resp); err != nil {
			return "", err
		}
		return resp.Message.Content, nil
	case ProviderOpenAI:
		var resp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		body := map[string]any{
			"model": l.cfg.Model, "temperature": 0.2, "messages": messages,
			"response_format": map[string]string{"type": "json_object"},
		}
		if err := l.post(ctx, l.cfg.BaseURL+"/chat/completions", body, &resp); err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", errors.New("empty response from model")
		}
		return resp.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("unsupported LLM provider %q", l.cfg.Provider)
}

func (l *LLM) post(ctx context.Context, url string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if l.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.cfg.APIKey)
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("LLM HTTP %d: %s", resp.StatusCode, truncate(string(data), 200))
	}
	return json.Unmarshal(data, out)
}

func (l *LLM) Status(ctx context.Context) Status {
	st := Status{Provider: l.cfg.Provider, Model: l.cfg.Model}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	switch l.cfg.Provider {
	case ProviderOllama:
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, l.cfg.BaseURL+"/api/tags", nil)
		resp, err := l.client.Do(req)
		if err != nil {
			st.Detail = "Ollama no responde en " + l.cfg.BaseURL
			return st
		}
		defer resp.Body.Close()
		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
			st.Detail = "respuesta inválida de Ollama"
			return st
		}
		for _, m := range tags.Models {
			if m.Name == l.cfg.Model || strings.TrimSuffix(m.Name, ":latest") == l.cfg.Model {
				st.Available = true
				st.Detail = "Modelo local disponible en Ollama"
				return st
			}
		}
		st.Detail = "El modelo " + l.cfg.Model + " no está descargado en Ollama"
	case ProviderOpenAI:
		st.Available = l.cfg.APIKey != ""
		st.Detail = "API compatible con OpenAI en " + l.cfg.BaseURL
		if !st.Available {
			st.Detail = "Falta LLM_API_KEY"
		}
	}
	return st
}

var percentRe = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*%`)

// Validate rejects model outputs that are empty, too long, or that quote
// percentages not present in the evidence (hallucinated numbers).
func Validate(out Explanation, f analytics.Finding) error {
	out.Reason = strings.TrimSpace(out.Reason)
	out.RecommendedAction = strings.TrimSpace(out.RecommendedAction)
	if out.Reason == "" || out.RecommendedAction == "" {
		return errors.New("empty reason or recommended_action")
	}
	if len(out.Reason) > 1200 || len(out.RecommendedAction) > 800 {
		return errors.New("text too long")
	}
	if len(out.EvidenceSummary) > 8 {
		return errors.New("too many evidence items")
	}
	allowed := allowedPercents(f)
	text := out.Reason + " " + out.RecommendedAction + " " + strings.Join(out.EvidenceSummary, " ")
	for _, m := range percentRe.FindAllStringSubmatch(text, -1) {
		v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		if err != nil {
			continue
		}
		if !nearAny(v, allowed, 1.5) {
			return fmt.Errorf("percentage %s%% is not supported by the evidence", m[1])
		}
	}
	return nil
}

func allowedPercents(f analytics.Finding) []float64 {
	ev := f.Evidence
	vals := []float64{ev.DeviationPct, ev.Current.VariationPct, ev.DataQuality.AffectedPct, f.Confidence * 100, 5, 100}
	for _, v := range ev.Variables {
		vals = append(vals, v.ChangePct)
	}
	return vals
}

func nearAny(v float64, allowed []float64, tol float64) bool {
	for _, a := range allowed {
		if math.Abs(math.Abs(a)-v) <= tol {
			return true
		}
	}
	return false
}

// extractJSON tolerates models that wrap the JSON in prose or code fences.
func extractJSON(s string) string {
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
