package config

import (
	"log"
	"log/slog"
	"net"
	"net/url"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	BotVersion string `env:"BOT_VER" env-default:"1.0.0"`
	BotToken   string `env:"TOKEN" env-required:"true"`
	Env        string `env:"ENV" env-required:"true"`
	GRpcPort   string `env:"GRPC_PORT" env-required:"true"`

	Postgres PGConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
}

type PGConfig struct {
	User     string `env:"PG_USER" env-required:"true"`
	Password string `env:"PG_PASS" env-required:"true"`
	Host     string `env:"PG_HOST" env-required:"true"`
	Port     string `env:"PG_PORT" env-required:"true"`
	Database string `env:"PG_DB" env-required:"true"`
}

type RedisConfig struct {
	Host     string `env:"REDIS_HOST" env-required:"true"`
	Port     string `env:"REDIS_PORT" env-required:"true"`
	Password string `env:"REDIS_PASS" env-required:"true"`
}

type RabbitMQConfig struct {
	User     string `env:"RMQ_USER" env-required:"true"`
	Password string `env:"RMQ_PASS" env-required:"true"`
	Host     string `env:"RMQ_HOST" env-default:"rabbitmq"`
	Port     string `env:"RMQ_PORT" env-default:"5672"`
}

func (r RabbitMQConfig) UrlRmq() string {
	host := r.Host
	if host == "" {
		host = "rabbitmq"
	}
	port := r.Port
	if port == "" {
		port = "5672"
	}
	u := &url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(r.User, r.Password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/",
	}
	return u.String()
}

func (p PGConfig) URL() string {
	u := &url.URL{
		Scheme:   "postgresql",
		User:     url.UserPassword(p.User, p.Password),
		Host:     net.JoinHostPort(p.Host, p.Port),
		Path:     p.Database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func (r RedisConfig) Addr() string {
	return net.JoinHostPort(r.Host, r.Port)
}

func LoadConfig() *Config {
	var cfg Config

	err := cleanenv.ReadConfig(".env", &cfg)
	if err != nil {
		slog.Info(".env file not found, reading from system env variables")
		if errEnv := cleanenv.ReadEnv(&cfg); errEnv != nil {
			log.Fatalf("Config error: %s", errEnv)
		}
	}
	slog.Info("Config loaded")
	return &cfg
}
