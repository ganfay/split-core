package config

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config holds runtime configuration for the notifier service.
type Config struct {
	Env      string `env:"ENV" env-default:"local"`
	GRpcHost string `env:"GRPC_HOST" env-default:"bot"`
	GRpcPort string `env:"GRPC_PORT" env-required:"true"`

	RabbitMQ RabbitMQConfig
}

// RabbitMQConfig holds AMQP connection and queue settings.
type RabbitMQConfig struct {
	User     string `env:"RMQ_USER" env-required:"true"`
	Password string `env:"RMQ_PASS" env-required:"true"`
	Host     string `env:"RMQ_HOST" env-default:"rabbitmq"`
	Port     string `env:"RMQ_PORT" env-default:"5672"`
	Queue    string `env:"RMQ_QUEUE" env-default:"test"`
}

// UrlRmq builds a properly escaped AMQP connection URI.
func (r RabbitMQConfig) UrlRmq() string {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "rabbitmq"
	}
	port := strings.TrimSpace(r.Port)
	if port == "" {
		port = "5672"
	}
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(r.User, r.Password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/",
	}
	return u.String()
}

// GRpcTarget returns the host:port gRPC target address for the bot core server.
func (c Config) GRpcTarget() string {
	host := strings.TrimSpace(c.GRpcHost)
	if host == "" {
		host = "bot"
	}
	port := strings.TrimPrefix(strings.TrimSpace(c.GRpcPort), ":")
	return net.JoinHostPort(host, port)
}

// Validate verifies all required fields and port formats.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config: nil config")
	}

	var errs []error

	if strings.TrimSpace(c.GRpcHost) == "" {
		errs = append(errs, errors.New("config: GRPC_HOST must not be empty"))
	}
	if _, err := parsePort(c.GRpcPort, true); err != nil {
		errs = append(errs, fmt.Errorf("config: invalid GRPC_PORT %q: %w", c.GRpcPort, err))
	}

	if strings.TrimSpace(c.RabbitMQ.User) == "" {
		errs = append(errs, errors.New("config: RMQ_USER must not be empty"))
	}
	if c.RabbitMQ.Password == "" {
		errs = append(errs, errors.New("config: RMQ_PASS must not be empty"))
	}
	if strings.TrimSpace(c.RabbitMQ.Host) == "" {
		errs = append(errs, errors.New("config: RMQ_HOST must not be empty"))
	}
	if _, err := parsePort(c.RabbitMQ.Port, false); err != nil {
		errs = append(errs, fmt.Errorf("config: invalid RMQ_PORT %q: %w", c.RabbitMQ.Port, err))
	}
	if strings.TrimSpace(c.RabbitMQ.Queue) == "" {
		errs = append(errs, errors.New("config: RMQ_QUEUE must not be empty"))
	}

	return errors.Join(errs...)
}

func parsePort(raw string, allowLeadingColon bool) (int, error) {
	s := strings.TrimSpace(raw)
	if allowLeadingColon {
		s = strings.TrimPrefix(s, ":")
	}
	if s == "" {
		return 0, errors.New("port is empty")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, errors.New("port must contain digits only")
		}
	}
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}
	return p, nil
}

// Load reads configuration from .env (if present) and environment variables, then validates it.
func Load() (*Config, error) {
	return LoadFromFile(".env")
}

// LoadFromFile reads configuration from envPath (if present) and environment variables, then validates it.
func LoadFromFile(envPath string) (*Config, error) {
	var cfg Config

	if envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			if err := cleanenv.ReadConfig(envPath, &cfg); err != nil {
				return nil, fmt.Errorf("config: read %s: %w", envPath, err)
			}
		} else if os.IsNotExist(err) {
			slog.Info(".env file not found, reading from system env variables")
			if err := cleanenv.ReadEnv(&cfg); err != nil {
				return nil, fmt.Errorf("config: read env: %w", err)
			}
		} else {
			return nil, fmt.Errorf("config: stat %s: %w", envPath, err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("config: read env: %w", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	slog.Info("Config loaded", "env", cfg.Env, "grpc_target", cfg.GRpcTarget(), "rmq_host", cfg.RabbitMQ.Host, "rmq_queue", cfg.RabbitMQ.Queue)
	return &cfg, nil
}

// LoadConfig loads configuration and exits the process if loading or validation fails.
func LoadConfig() *Config {
	cfg, err := Load()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	return cfg
}
