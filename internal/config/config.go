package config

import (
	"fmt"
	"log/slog"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	BotVersion string `env:"BOT_VER" env-default:"1.0.0"`
	BotToken   string `env:"TOKEN" env-required:"true"`

	Postgres PGConfig
	Redis    RedisConfig
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

func (p PGConfig) URL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", p.User, p.Password, p.Host, p.Port, p.Database)
}

func (r RedisConfig) URL() string {
	return fmt.Sprintf("redis://%s:%s", r.Host, r.Port)
}

func LoadConfig() *Config {
	var config Config
	if err := cleanenv.ReadConfig(".env", &config); err != nil {
		slog.Error("Error loading config", "error", err)
	}
	return &config
}
