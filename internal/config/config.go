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
	App    AppConfig    `yaml:"app"`
	DB     DBConfig     `yaml:"db"`
	Redis  RedisConfig  `yaml:"redis"`
	Kafka  KafkaConfig  `yaml:"kafka"`
	Server ServerConfig `yaml:"server"`
	Mock   MockConfig   `yaml:"mock"`
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
