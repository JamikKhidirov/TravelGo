package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

const minimalConfig = `
app:
  name: test
  env: development
db:
  host: localhost
  port: 5432
  user: travel
  password: secret
  name: travel_db
  sslmode: disable
`

func TestLoadAppliesPoolDefaults(t *testing.T) {
	path := writeConfig(t, minimalConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DB.MaxOpenConns != 25 {
		t.Fatalf("MaxOpenConns = %d, want 25", cfg.DB.MaxOpenConns)
	}
	if cfg.DB.MaxIdleConns != 10 {
		t.Fatalf("MaxIdleConns = %d, want 10", cfg.DB.MaxIdleConns)
	}
	if got := cfg.ConnMaxLifetimeDuration(); got != 5*time.Minute {
		t.Fatalf("ConnMaxLifetimeDuration = %v, want 5m", got)
	}
}

func TestLoadReadsPoolFromYAML(t *testing.T) {
	path := writeConfig(t, minimalConfig+`
  max_open_conns: 42
  max_idle_conns: 7
  conn_max_lifetime: 90s
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DB.MaxOpenConns != 42 || cfg.DB.MaxIdleConns != 7 {
		t.Fatalf("pool = %d/%d, want 42/7", cfg.DB.MaxOpenConns, cfg.DB.MaxIdleConns)
	}
	if got := cfg.ConnMaxLifetimeDuration(); got != 90*time.Second {
		t.Fatalf("ConnMaxLifetimeDuration = %v, want 90s", got)
	}
}

func TestLoadEnvOverridesPool(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "60")
	t.Setenv("DB_MAX_IDLE_CONNS", "15")
	t.Setenv("DB_CONN_MAX_LIFETIME", "2m")
	path := writeConfig(t, minimalConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DB.MaxOpenConns != 60 {
		t.Fatalf("MaxOpenConns = %d, want 60", cfg.DB.MaxOpenConns)
	}
	if cfg.DB.MaxIdleConns != 15 {
		t.Fatalf("MaxIdleConns = %d, want 15", cfg.DB.MaxIdleConns)
	}
	if got := cfg.ConnMaxLifetimeDuration(); got != 2*time.Minute {
		t.Fatalf("ConnMaxLifetimeDuration = %v, want 2m", got)
	}
}

func TestLoadClampsIdleToOpen(t *testing.T) {
	path := writeConfig(t, minimalConfig+`
  max_open_conns: 5
  max_idle_conns: 50
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DB.MaxIdleConns != 5 {
		t.Fatalf("MaxIdleConns = %d, want clamped to 5", cfg.DB.MaxIdleConns)
	}
}

func TestInvalidLifetimeFallsBack(t *testing.T) {
	path := writeConfig(t, minimalConfig+`
  conn_max_lifetime: not-a-duration
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.ConnMaxLifetimeDuration(); got != 5*time.Minute {
		t.Fatalf("ConnMaxLifetimeDuration = %v, want fallback 5m", got)
	}
}

func TestLoadRejectsIncompleteDB(t *testing.T) {
	path := writeConfig(t, "app:\n  name: test\n")
	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error for missing db config")
	}
}
