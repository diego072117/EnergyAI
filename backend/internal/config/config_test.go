package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8080" || c.LLMProvider != "none" || c.BaselineDays != 7 || c.StepDelay != 450*time.Millisecond {
		t.Errorf("unexpected defaults %+v", c)
	}
}

func TestDotEnvDoesNotOverrideEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# comment\nPORT=9000\nexport LLM_PROVIDER=ollama\nLLM_MODEL=\"llama3\"\nDEMO_NAME=Ana\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEMO_NAME", "Desde entorno")
	for _, k := range []string{"PORT", "LLM_PROVIDER", "LLM_MODEL"} {
		k := k
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "9000" || c.LLM.Provider != "ollama" || c.LLM.Model != "llama3" || c.LLM.BaseURL != "http://localhost:11434" {
		t.Errorf("dotenv not applied: %+v", c)
	}
	if c.DemoName != "Desde entorno" {
		t.Errorf("environment must win over .env, got %q", c.DemoName)
	}
}

func TestInvalidValues(t *testing.T) {
	cases := map[string]string{
		"LLM_PROVIDER":           "gpt",
		"ANALYSIS_STEP_DELAY_MS": "-1",
		"BASELINE_DAYS":          "0",
		"JWT_TTL":                "forever",
		"JWT_SECRET":             "short",
		"LOG_LEVEL":              "loud",
	}
	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			t.Setenv(k, v)
			if _, err := Load(""); err == nil {
				t.Errorf("%s=%s should fail", k, v)
			}
		})
	}
}

func TestOpenAIRequiresKey(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("LLM_API_KEY", "")
	if _, err := Load(""); err == nil {
		t.Fatal("expected error without API key")
	}
	t.Setenv("LLM_API_KEY", "k")
	c, err := Load("")
	if err != nil || c.LLM.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("got %+v %v", c.LLM, err)
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" a, ,b ,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v", got)
	}
}
