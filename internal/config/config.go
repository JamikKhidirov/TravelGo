package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	DB        DBConfig        `yaml:"db"`
	Redis     RedisConfig     `yaml:"redis"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	Server    ServerConfig    `yaml:"server"`
	Mock      MockConfig      `yaml:"mock"`
	Security  SecurityConfig  `yaml:"security"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

type AppConfig struct {
	Name         string `yaml:"name"`
	Env          string `yaml:"env"`
	Port         int    `yaml:"port"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

type DBConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Name            string `yaml:"name"`
	SSLMode         string `yaml:"sslmode"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type KafkaConfig struct {
	Brokers           []string `yaml:"brokers"`
	TopicTravelEvents string   `yaml:"topic_travel_events"`
	TopicSync         string   `yaml:"topic_sync"`
	ConsumerGroup     string   `yaml:"consumer_group"`
}

type ServerConfig struct {
	Origin string `yaml:"origin"`
}

type MockConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
}

// SecurityConfig controls proxy trust, auth hardening and data retention.
type SecurityConfig struct {
	TrustedProxies        []string `yaml:"trusted_proxies"`
	AllowHeaderAuth       bool     `yaml:"allow_header_auth"`
	EventRetentionDays    int      `yaml:"event_retention_days"`
	SessionCleanupMinutes int      `yaml:"session_cleanup_minutes"`
	MaxFailedLogins       int      `yaml:"max_failed_logins"`
	LockoutMinutes        int      `yaml:"lockout_minutes"`
}

// RateLimitConfig enables per-IP request limiting.
type RateLimitConfig struct {
	Enabled               *bool `yaml:"enabled"`
	RequestsPerMinute     int   `yaml:"requests_per_minute"`
	AuthRequestsPerMinute int   `yaml:"auth_requests_per_minute"`
}

// RateLimitEnabled reports whether limiting is on (default: on).
func (c *Config) RateLimitEnabled() bool {
	if c.RateLimit.Enabled == nil {
		return true
	}
	return *c.RateLimit.Enabled
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.applyEnvOverrides()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("APP_ENV"); v != "" {
		c.App.Env = v
	}
	if v := os.Getenv("APP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.App.Port = n
		}
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		c.DB.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.DB.Port = n
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		c.DB.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		c.DB.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		c.DB.Name = v
	}
	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.DB.MaxOpenConns = n
		}
	}
	if v := os.Getenv("DB_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.DB.MaxIdleConns = n
		}
	}
	if v := os.Getenv("DB_CONN_MAX_LIFETIME"); v != "" {
		c.DB.ConnMaxLifetime = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.Redis.Addr = v
	}
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		c.Kafka.Brokers = strings.Split(v, ",")
	}
	if v := os.Getenv("TRUSTED_PROXIES"); v != "" {
		c.Security.TrustedProxies = strings.Split(v, ",")
	}
	if v := os.Getenv("ALLOW_HEADER_AUTH"); v != "" {
		c.Security.AllowHeaderAuth = v == "true" || v == "1"
	}
	if v := os.Getenv("AUTH_EVENT_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Security.EventRetentionDays = n
		}
	}
	if v := os.Getenv("SESSION_CLEANUP_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Security.SessionCleanupMinutes = n
		}
	}
	if v := os.Getenv("AUTH_MAX_FAILED_LOGINS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Security.MaxFailedLogins = n
		}
	}
	if v := os.Getenv("AUTH_LOCKOUT_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Security.LockoutMinutes = n
		}
	}
	if v := os.Getenv("RATE_LIMIT_ENABLED"); v != "" {
		b := v == "true" || v == "1"
		c.RateLimit.Enabled = &b
	}
	if v := os.Getenv("RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.RateLimit.RequestsPerMinute = n
		}
	}
	if v := os.Getenv("AUTH_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.RateLimit.AuthRequestsPerMinute = n
		}
	}
}

func (c *Config) validate() error {
	if c.DB.Host == "" || c.DB.User == "" || c.DB.Name == "" {
		return fmt.Errorf("db config incomplete")
	}
	if c.App.Port == 0 {
		c.App.Port = 8080
	}
	if c.DB.MaxOpenConns <= 0 {
		c.DB.MaxOpenConns = 25
	}
	if c.DB.MaxIdleConns <= 0 {
		c.DB.MaxIdleConns = 10
	}
	if c.DB.MaxIdleConns > c.DB.MaxOpenConns {
		c.DB.MaxIdleConns = c.DB.MaxOpenConns
	}
	if c.DB.ConnMaxLifetime == "" {
		c.DB.ConnMaxLifetime = "300s"
	}
	if c.Security.EventRetentionDays <= 0 {
		c.Security.EventRetentionDays = 180
	}
	if c.Security.SessionCleanupMinutes <= 0 {
		c.Security.SessionCleanupMinutes = 30
	}
	if c.Security.MaxFailedLogins <= 0 {
		c.Security.MaxFailedLogins = 5
	}
	if c.Security.LockoutMinutes <= 0 {
		c.Security.LockoutMinutes = 15
	}
	if c.RateLimit.RequestsPerMinute <= 0 {
		c.RateLimit.RequestsPerMinute = 120
	}
	if c.RateLimit.AuthRequestsPerMinute <= 0 {
		c.RateLimit.AuthRequestsPerMinute = 10
	}
	// Header-based identity is a development-only shortcut.
	if c.App.Env == "production" {
		c.Security.AllowHeaderAuth = false
	}
	return nil
}

// ConnMaxLifetimeDuration parses the configured pool connection lifetime.
func (c *Config) ConnMaxLifetimeDuration() time.Duration {
	d, err := time.ParseDuration(c.DB.ConnMaxLifetime)
	if err != nil || d <= 0 {
		return 5 * time.Minute
	}
	return d
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DB.Host, c.DB.Port, c.DB.User, c.DB.Password, c.DB.Name, c.DB.SSLMode)
}

func (c *Config) AppTimeout(duration string) time.Duration {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return 10 * time.Second
	}
	return d
}
