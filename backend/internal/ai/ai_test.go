package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/ingest"
)

var (
	datasetOnce     sync.Once
	datasetFindings []analytics.Finding
)

func findings(t *testing.T) []analytics.Finding {
	t.Helper()
	datasetOnce.Do(func() {
		dir := filepath.Join("..", "..", "data")
		rs, err := ingest.ReadReadingsFile(filepath.Join(dir, "readings.csv"))
		if err != nil {
			t.Fatal(err)
		}
		evs, err := ingest.ReadEventsFile(filepath.Join(dir, "events.csv"))
		if err != nil {
			t.Fatal(err)
		}
		datasetFindings = analytics.Run(analytics.DefaultConfig(), analytics.BuildInputs(rs, evs)).Findings
	})
	return datasetFindings
}

func byType(t *testing.T, typ domain.AnomalyType) analytics.Finding {
	t.Helper()
	for _, f := range findings(t) {
		if f.Type == typ {
			return f
		}
	}
	t.Fatalf("no %s finding", typ)
	return analytics.Finding{}
}

func TestTemplateExplanations(t *testing.T) {
	cases := []struct {
		typ          domain.AnomalyType
		reasonHas    []string
		actionHas    string
		minEvidences int
	}{
		{domain.AnomalyReal, []string{"+110,4%", "sin ningún evento operativo", "factor de potencia"}, "inspección en sitio", 3},
		{domain.AnomalyExplainable, []string{"New production line activated"}, "actualizar el baseline", 2},
		{domain.AnomalyFalsePositive, []string{"Scheduled maintenance", "volvió a su nivel normal"}, "No escalar", 2},
		{domain.AnomalyDataQuality, []string{"voltaje fuera de rango", "consumo se mantiene estable"}, "no confiables", 2},
	}
	for _, c := range cases {
		exp, err := Template{}.Explain(context.Background(), byType(t, c.typ))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range c.reasonHas {
			if !strings.Contains(exp.Reason, s) {
				t.Errorf("%s reason missing %q: %s", c.typ, s, exp.Reason)
			}
		}
		if !strings.Contains(exp.RecommendedAction, c.actionHas) {
			t.Errorf("%s action missing %q: %s", c.typ, c.actionHas, exp.RecommendedAction)
		}
		if len(exp.EvidenceSummary) < c.minEvidences || exp.Source != SourceTemplate {
			t.Errorf("%s: evidence %d source %s", c.typ, len(exp.EvidenceSummary), exp.Source)
		}
		// Every template must pass the same guardrails applied to the LLM.
		if err := Validate(exp, byType(t, c.typ)); err != nil {
			t.Errorf("%s template fails validation: %v", c.typ, err)
		}
	}
}

func TestTemplateUnknownType(t *testing.T) {
	if _, err := (Template{}).Explain(context.Background(), analytics.Finding{Type: "OTHER"}); err == nil {
		t.Fatal("expected error")
	}
}

func fakeOllama(t *testing.T, content string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"name": "qwen2.5:7b"}}})
		case "/api/chat":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["format"] == nil {
				t.Errorf("request must include a JSON schema format: %v", err)
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"role": "assistant", "content": content}})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestLLMOllamaValidOutput(t *testing.T) {
	out := `{"reason":"El consumo subió 110,4% sin evento operativo.","recommended_action":"Inspeccionar la instalación hoy.","evidence_summary":["Corriente duplicada","FP en caída"]}`
	srv := fakeOllama(t, "```json\n"+out+"\n```", http.StatusOK)
	defer srv.Close()
	llm := NewLLM(LLMConfig{Provider: ProviderOllama, BaseURL: srv.URL + "/", Model: "qwen2.5:7b"})
	exp, err := llm.Explain(context.Background(), byType(t, domain.AnomalyReal))
	if err != nil {
		t.Fatal(err)
	}
	// The evidence list is always the deterministic one computed by the engine.
	if exp.Source != "LLM:qwen2.5:7b" || !strings.Contains(exp.Reason, "110,4%") || len(exp.EvidenceSummary) < 3 {
		t.Errorf("unexpected explanation %+v", exp)
	}
	if st := llm.Status(context.Background()); !st.Available {
		t.Errorf("status %+v", st)
	}
}

func TestLLMRejectsHallucinatedPercentages(t *testing.T) {
	out := `{"reason":"El consumo subió 250% por una fuga.","recommended_action":"Revisar.","evidence_summary":[]}`
	srv := fakeOllama(t, out, http.StatusOK)
	defer srv.Close()
	llm := NewLLM(LLMConfig{Provider: ProviderOllama, BaseURL: srv.URL, Model: "qwen2.5:7b"})
	if _, err := llm.Explain(context.Background(), byType(t, domain.AnomalyReal)); err == nil {
		t.Fatal("expected hallucinated percentage to be rejected")
	}
}

func TestFallbackToTemplate(t *testing.T) {
	for name, srv := range map[string]*httptest.Server{
		"invalid json": fakeOllama(t, "no es json", http.StatusOK),
		"http error":   fakeOllama(t, "", http.StatusInternalServerError),
		"empty fields": fakeOllama(t, `{"reason":"","recommended_action":"","evidence_summary":[]}`, http.StatusOK),
	} {
		llm := NewLLM(LLMConfig{Provider: ProviderOllama, BaseURL: srv.URL, Model: "qwen2.5:7b"})
		exp, err := WithFallback{Primary: llm, Fallback: Template{}}.Explain(context.Background(), byType(t, domain.AnomalyReal))
		srv.Close()
		if err != nil || exp.Source != SourceTemplate {
			t.Errorf("%s: expected template fallback, got %+v (%v)", name, exp, err)
		}
	}
}

func TestLLMOpenAICompatible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		content := `{"reason":"Caída explicada por mantenimiento programado.","recommended_action":"No escalar.","evidence_summary":["Duración coincide"]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}}}})
	}))
	defer srv.Close()
	llm := NewLLM(LLMConfig{Provider: ProviderOpenAI, BaseURL: srv.URL + "/v1", Model: "gpt-x", APIKey: "test-key"})
	exp, err := llm.Explain(context.Background(), byType(t, domain.AnomalyFalsePositive))
	if err != nil || exp.Source != "LLM:gpt-x" {
		t.Fatalf("got %+v, %v", exp, err)
	}
	if st := llm.Status(context.Background()); !st.Available {
		t.Errorf("status %+v", st)
	}
	if st := NewLLM(LLMConfig{Provider: ProviderOpenAI}).Status(context.Background()); st.Available {
		t.Error("without key the provider must not be available")
	}
}

func TestOllamaStatusUnavailable(t *testing.T) {
	st := NewLLM(LLMConfig{Provider: ProviderOllama, BaseURL: "http://127.0.0.1:1", Model: "m"}).Status(context.Background())
	if st.Available {
		t.Fatal("expected unavailable")
	}
	srv := fakeOllama(t, "", http.StatusOK)
	defer srv.Close()
	if st := NewLLM(LLMConfig{Provider: ProviderOllama, BaseURL: srv.URL, Model: "llama3"}).Status(context.Background()); st.Available {
		t.Error("model not pulled must be reported unavailable")
	}
}

func TestUnsupportedProvider(t *testing.T) {
	if _, err := NewLLM(LLMConfig{Provider: "x"}).Explain(context.Background(), byType(t, domain.AnomalyReal)); err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractJSON(t *testing.T) {
	if got := extractJSON("Aquí está:\n{\"a\":1}\ngracias"); got != `{"a":1}` {
		t.Errorf("got %q", got)
	}
}

// TestOllamaLive runs against a real local Ollama. Enable with LLM_LIVE=1.
func TestOllamaLive(t *testing.T) {
	if os.Getenv("LLM_LIVE") != "1" {
		t.Skip("set LLM_LIVE=1 to run against a local Ollama")
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "qwen2.5:7b"
	}
	llm := NewLLM(LLMConfig{Provider: ProviderOllama, BaseURL: "http://localhost:11434", Model: model})
	for _, f := range findings(t) {
		exp, err := llm.Explain(context.Background(), f)
		if err != nil {
			t.Errorf("%s: %v", f.MeterID, err)
			continue
		}
		t.Logf("%s %s\n  reason: %s\n  action: %s\n  evidence: %v", f.MeterID, f.Type, exp.Reason, exp.RecommendedAction, exp.EvidenceSummary)
	}
}
