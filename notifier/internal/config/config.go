package config

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	GRpcPort string `env:"GRPC_PORT" env-required:"true"`

	RabbitMQ RabbitMQConfig
}

type RabbitMQConfig struct {
	User     string `env:"RMQ_USER" env-required:"true"`
	Password string `env:"RMQ_PASS" env-required:"true"`
}

func (r RabbitMQConfig) UrlRmq() string {
	return fmt.Sprintf("amqp://%s:%s@rabbitmq:5672/", r.User, r.Password)
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
