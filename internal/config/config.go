package config

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Server      HTTPConfig        `yaml:"server"`
	Logger      LoggerConfig      `yaml:"logger"`
	Postgres    PostgresConfig    `yaml:"postgres"`
	Kafka       KafkaConfig       `yaml:"kafka"`
	AlertsKafka AlertsKafkaConfig `yaml:"alerts_kafka"`
	SMTP        SMTPConfig        `yaml:"smtp"`
	Worker      WorkerConfig      `yaml:"worker"`
	Auth        AuthConfig        `yaml:"auth"`
}

type AuthConfig struct {
	Secret string `env:"JWT_SECRET" env-default:"my-super-secret-key" yaml:"secret"`
}

type HTTPConfig struct {
	Host    string `env:"HTTP_HOST" env-default:"0.0.0.0" yaml:"host"`
	Port    string `env:"HTTP_PORT" env-default:"8080" yaml:"port"`
	Mode    string `env:"GIN_MODE" env-default:"release"`
	Timeout struct {
		Server time.Duration `yaml:"server"`
		Write  time.Duration `yaml:"write"`
		Read   time.Duration `yaml:"read"`
		Idle   time.Duration `yaml:"idle"`
	} `yaml:"timeout"`
}

type LoggerConfig struct {
	Path string `env:"LOGGER_CONFIG_PATH" env-default:"config/logger.json"`
}

type PostgresConfig struct {
	Host           string `env:"POSTGRES_HOST" env-default:"localhost"`
	Port           string `env:"POSTGRES_PORT" env-default:"5432"`
	Username       string `env:"POSTGRES_USER" env-default:"postgres"`
	Password       string `env:"POSTGRES_PASSWORD" env-default:"postgres"`
	Database       string `env:"POSTGRES_DB" env-default:"postgres"`
	SSLMode        string `env:"POSTGRES_SSL_MODE" env-default:"disable"`
	MigrationsPath string `env:"GOOSE_MIGRATION_DIR" env-default:"./migrations"`

	MaxOpenConns    int32         `env-default:"25" yaml:"max_open_conns"`
	MaxIdleConns    int32         `env-default:"5" yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `env-default:"1h" yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `env-default:"30m" yaml:"conn_max_idle_time"`
	Timeout         time.Duration `env-default:"5s" yaml:"timeout"`
}

type KafkaConfig struct {
	Brokers       []string      `env:"KAFKA_BROKERS" env-default:"localhost:9092" env-separator:"," yaml:"brokers"`
	Topic         string        `env:"KAFKA_TOPIC" env-default:"notifications.email.send" yaml:"topic"`
	DLQTopic      string        `env:"KAFKA_DLQ_TOPIC" env-default:"notifications.email.dlq" yaml:"dlq_topic"`
	GroupID       string        `env:"KAFKA_GROUP_ID" env-default:"notification-email-sender" yaml:"group_id"`
	ReadMinBytes  int           `env:"KAFKA_READ_MIN_BYTES" env-default:"1024" yaml:"read_min_bytes"`
	ReadMaxBytes  int           `env:"KAFKA_READ_MAX_BYTES" env-default:"1048576" yaml:"read_max_bytes"`
	CommitTimeout time.Duration `env:"KAFKA_COMMIT_TIMEOUT" env-default:"5s" yaml:"commit_timeout"`
}

type AlertsKafkaConfig struct {
	Brokers       []string      `env:"ALERTS_KAFKA_BROKERS" env-separator:"," yaml:"brokers"`
	Topic         string        `env:"ALERTS_KAFKA_TOPIC" yaml:"topic"`
	DLQTopic      string        `env:"ALERTS_KAFKA_DLQ_TOPIC" yaml:"dlq_topic"`
	GroupID       string        `env:"ALERTS_KAFKA_GROUP_ID" yaml:"group_id"`
	ReadMinBytes  int           `env:"ALERTS_KAFKA_READ_MIN_BYTES" yaml:"read_min_bytes"`
	ReadMaxBytes  int           `env:"ALERTS_KAFKA_READ_MAX_BYTES" yaml:"read_max_bytes"`
	CommitTimeout time.Duration `env:"ALERTS_KAFKA_COMMIT_TIMEOUT" yaml:"commit_timeout"`
}

func (c AlertsKafkaConfig) ToKafkaConfig(fallback KafkaConfig) KafkaConfig {
	cfg := KafkaConfig{
		Brokers:       c.Brokers,
		Topic:         c.Topic,
		DLQTopic:      c.DLQTopic,
		GroupID:       c.GroupID,
		ReadMinBytes:  c.ReadMinBytes,
		ReadMaxBytes:  c.ReadMaxBytes,
		CommitTimeout: c.CommitTimeout,
	}

	if len(cfg.Brokers) == 0 {
		cfg.Brokers = fallback.Brokers
	}
	if cfg.Topic == "" {
		cfg.Topic = "notifications.alerts.created"
	}
	if cfg.DLQTopic == "" {
		cfg.DLQTopic = "notifications.alerts.dlq"
	}
	if cfg.GroupID == "" {
		cfg.GroupID = "notification-alerts-inbox"
	}
	if cfg.ReadMinBytes == 0 {
		cfg.ReadMinBytes = fallback.ReadMinBytes
	}
	if cfg.ReadMaxBytes == 0 {
		cfg.ReadMaxBytes = fallback.ReadMaxBytes
	}
	if cfg.CommitTimeout == 0 {
		cfg.CommitTimeout = fallback.CommitTimeout
	}

	return cfg
}

type SMTPConfig struct {
	Host               string        `env:"SMTP_HOST" env-default:"localhost" yaml:"host"`
	Port               int           `env:"SMTP_PORT" env-default:"1025" yaml:"port"`
	Username           string        `env:"SMTP_USERNAME" yaml:"username"`
	Password           string        `env:"SMTP_PASSWORD" yaml:"password"`
	From               string        `env:"SMTP_FROM" env-default:"no-reply@example.com" yaml:"from"`
	UseTLS             bool          `env:"SMTP_USE_TLS" env-default:"false" yaml:"use_tls"`
	StartTLS           bool          `env:"SMTP_START_TLS" env-default:"false" yaml:"start_tls"`
	InsecureSkipVerify bool          `env:"SMTP_INSECURE_SKIP_VERIFY" env-default:"false" yaml:"insecure_skip_verify"`
	DialTimeout        time.Duration `env:"SMTP_DIAL_TIMEOUT" env-default:"10s" yaml:"dial_timeout"`
	SendTimeout        time.Duration `env:"SMTP_SEND_TIMEOUT" env-default:"15s" yaml:"send_timeout"`
	DefaultSubject     string        `env:"SMTP_DEFAULT_SUBJECT" env-default:"Notification" yaml:"default_subject"`
}

type WorkerConfig struct {
	MaxRetries        int           `env:"WORKER_MAX_RETRIES" env-default:"3" yaml:"max_retries"`
	InitialBackoff    time.Duration `env:"WORKER_INITIAL_BACKOFF" env-default:"1s" yaml:"initial_backoff"`
	MaxBackoff        time.Duration `env:"WORKER_MAX_BACKOFF" env-default:"20s" yaml:"max_backoff"`
	LoopErrorBackoff  time.Duration `env:"WORKER_LOOP_ERROR_BACKOFF" env-default:"2s" yaml:"loop_error_backoff"`
	ProcessingTimeout time.Duration `env:"WORKER_PROCESSING_TIMEOUT" env-default:"30s" yaml:"processing_timeout"`
	LockTTL           time.Duration `env:"WORKER_LOCK_TTL" env-default:"2m" yaml:"lock_ttl"`
}

const (
	defaultConfigPath = "config/config.yaml"
	Path              = "CONFIG_PATH"
)

func MustLoad() *Config {
	cfg, err := load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	return cfg
}

func load() (*Config, error) {
	if _, err := os.Stat(".env"); err == nil {
		if loadErr := godotenv.Load(); loadErr != nil {
			return nil, fmt.Errorf("failed to load .env: %w", loadErr)
		}
	}

	configPath := getConfigPath()
	if configPath != "" {
		fileInfo, err := os.Stat(configPath)

		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file %s does not exist", configPath)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to access config file: %w", err)
		}

		if fileInfo.IsDir() {
			return nil, fmt.Errorf("config path is a directory, not a file: %s", configPath)
		}
	}

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("failed to read env vars: %w", err)
	}

	return &cfg, nil
}

func getConfigPath() string {
	configPath := os.Getenv(Path)
	if configPath == "" {
		configPath = defaultConfigPath
	}

	return configPath
}

func (h *HTTPConfig) GetAddr() string {
	return net.JoinHostPort(h.Host, h.Port)
}

func (s *SMTPConfig) Address() string {
	return net.JoinHostPort(s.Host, fmt.Sprintf("%d", s.Port))
}
