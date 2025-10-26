package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// config holds all application configuration
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Processing ProcessingConfig `mapstructure:"processing"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Window     WindowConfig     `mapstructure:"window"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// ProcessingConfig holds event processing configuration
type ProcessingConfig struct {
	WorkerCount   int           `mapstructure:"worker_count"`
	BufferSize    int           `mapstructure:"buffer_size"`
	BatchSize     int           `mapstructure:"batch_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
}

// storageConfig holds storage configuration
type StorageConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

// PostgresConfig holds Postgres database configuration
type PostgresConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	Database       string `mapstructure:"database"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
	MaxConnections int    `mapstructure:"max_connections"`
}

// RedisConfig holds Redis database configuration
type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// WindowsConfig holds time window configuration
type WindowConfig struct {
	Tumbling []WindowSpec `mapstructure:"tumbling"`
	Sliding  []WindowSpec `mapstructure:"sliding"`
}

// WindowSpec defines a time window specification
type WindowSpec struct {
	Size    time.Duration `mapstructure:"size"`
	Step    time.Duration `mapstructure:"step"`
	Metrics []string      `mapstructure:"metrics"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
}

// PrometheusConfig holds Prometheus monitoring configuration
type PrometheusConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

// Load reads configuration from file
func Load(ConfigPath string) (*Config, error) {
	viper.SetConfigFile(ConfigPath)
	viper.SetConfigType("yaml")

	//SET DEFAULTS
	setDefaults()

	//read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	//unmarshal config
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	//validate config
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil

}

// setDefaults sets default configuration values
func setDefaults() {
	// server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")
	viper.SetDefault("server.shutdown_timeout", "10s")

	// processing defaults
	viper.SetDefault("processing.worker_count", 10)
	viper.SetDefault("processing.buffer_size", 1000)
	viper.SetDefault("processing.batch_size", 100)
	viper.SetDefault("processing.flush_interval", "5s")

	//Storage defaults
	viper.SetDefault("storage.postgres.host", "localhost")
	viper.SetDefault("storage.postgres.port", 5432)
	viper.SetDefault("storage.postgres.database", "analytics")
	viper.SetDefault("storage.postgres.user", "postgres")
	viper.SetDefault("storage.postgres.max_connections", 25)

	viper.SetDefault("storage.redis.address", "localhost:6379")
	viper.SetDefault("storage.redis.db", 0)
	viper.SetDefault("storage.redis.pool_size", 50)

	//logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")

	//Monitoring defaults
	viper.SetDefault("monitoring.prometheus.enabled", true)
	viper.SetDefault("monitoring.prometheus.port", 9090)
	viper.SetDefault("monitoring.prometheus.path", "/metrics")
}

// validate checks if the configuration values are valid
func (c *Config) validate() error {
	//validate server config
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("Invalid server port: %d", c.Server.Port)
	}

	//valudate processing config
	if c.Processing.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be at least 1")
	}
	if c.Processing.BufferSize < 100 {
		return fmt.Errorf("buffer size must be at least 100")
	}

	//validate storage config
	if c.Storage.Postgres.Database == "" {
		return fmt.Errorf("postgres database name is required")
	}
	if c.Storage.Postgres.User == "" {
		return fmt.Errorf("postgres user is required")
	}

	//validate logging config
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}
	return nil
}

// GetPostgresConnectionString builds the Postgres connection string
func (c *Config) GetPostgresConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
		c.Storage.Postgres.Host,
		c.Storage.Postgres.Port,
		c.Storage.Postgres.Database,
		c.Storage.Postgres.User,
		c.Storage.Postgres.Password,
	)
}

// GetServerAddress returns the server address in "host:port" format
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
