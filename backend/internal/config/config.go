package config

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"energyai/internal/ai"
)

type Config struct {
	Port         string
	DatabaseURL  string
	DataDir      string
	JWTSecret    string
	JWTTTL       time.Duration
	DemoEmail    string
	DemoPassword string
	DemoName     string
	CORSOrigins  []string
	StepDelay    time.Duration
	BaselineDays int
	LogLevel     slog.Level
	LLMProvider  string // none | ollama | openai
	LLM          ai.LLMConfig
}

// Load reads the configuration. Values already present in the environment
// take precedence over the .env file.
func Load(envFile string) (Config, error) {
	if envFile != "" {
		if err := loadDotEnv(envFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Config{}, err
		}
	}
	c := Config{
		Port:         get("PORT", "8080"),
		DatabaseURL:  get("DATABASE_URL", "postgres://energy:energy@localhost:5432/energyai?sslmode=disable"),
		DataDir:      get("DATA_DIR", "./data"),
		JWTSecret:    get("JWT_SECRET", "dev-secret-change-me"),
		DemoEmail:    get("DEMO_EMAIL", "demo@energyai.local"),
		DemoPassword: get("DEMO_PASSWORD", "demo1234"),
		DemoName:     get("DEMO_NAME", "Operador Demo"),
		CORSOrigins:  splitList(get("CORS_ORIGINS", "http://localhost:5173")),
		LLMProvider:  strings.ToLower(get("LLM_PROVIDER", "none")),
	}
	var err error
	if c.JWTTTL, err = time.ParseDuration(get("JWT_TTL", "12h")); err != nil {
		return c, fmt.Errorf("JWT_TTL: %w", err)
	}
	delay, err := strconv.Atoi(get("ANALYSIS_STEP_DELAY_MS", "450"))
	if err != nil || delay < 0 {
		return c, fmt.Errorf("ANALYSIS_STEP_DELAY_MS must be a non-negative integer")
	}
	c.StepDelay = time.Duration(delay) * time.Millisecond
	if c.BaselineDays, err = strconv.Atoi(get("BASELINE_DAYS", "7")); err != nil || c.BaselineDays < 1 {
		return c, fmt.Errorf("BASELINE_DAYS must be a positive integer")
	}
	if err := c.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		return c, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	timeout, err := time.ParseDuration(get("LLM_TIMEOUT", "90s"))
	if err != nil {
		return c, fmt.Errorf("LLM_TIMEOUT: %w", err)
	}
	c.LLM = ai.LLMConfig{
		Provider: c.LLMProvider,
		Model:    get("LLM_MODEL", "qwen2.5:7b"),
		APIKey:   os.Getenv("LLM_API_KEY"),
		Timeout:  timeout,
	}
	switch c.LLMProvider {
	case "none", "":
		c.LLMProvider = "none"
	case ai.ProviderOllama:
		c.LLM.BaseURL = get("LLM_BASE_URL", "http://localhost:11434")
	case ai.ProviderOpenAI:
		c.LLM.BaseURL = get("LLM_BASE_URL", "https://api.openai.com/v1")
		if c.LLM.APIKey == "" {
			return c, errors.New("LLM_PROVIDER=openai requires LLM_API_KEY")
		}
	default:
		return c, fmt.Errorf("LLM_PROVIDER must be none, ollama or openai (got %q)", c.LLMProvider)
	}
	if len(c.JWTSecret) < 8 {
		return c, errors.New("JWT_SECRET must have at least 8 characters")
	}
	return c, nil
}

func get(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(strings.TrimPrefix(k, "export "))
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
