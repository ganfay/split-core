package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigURLs(t *testing.T) {
	pgTests := []struct {
		name string
		cfg  PGConfig
		want string
	}{
		{
			name: "standard",
			cfg:  PGConfig{User: "app", Password: "secret", Host: "db", Port: "5432", Database: "splitcore"},
			want: "postgresql://app:secret@db:5432/splitcore?sslmode=disable",
		},
		{
			name: "password with special chars escaped",
			cfg:  PGConfig{User: "app", Password: "p@ss:w/rd#1", Host: "db", Port: "5432", Database: "splitcore"},
			want: "postgresql://app:p%40ss%3Aw%2Frd%231@db:5432/splitcore?sslmode=disable",
		},
	}
	for _, tc := range pgTests {
		t.Run("PG_"+tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cfg.URL())
		})
	}

	rmqTests := []struct {
		name string
		cfg  RabbitMQConfig
		want string
	}{
		{
			name: "default host and port",
			cfg:  RabbitMQConfig{User: "guest", Password: "guest"},
			want: "amqp://guest:guest@rabbitmq:5672/",
		},
		{
			name: "custom host and special password",
			cfg:  RabbitMQConfig{User: "u", Password: "p@ss", Host: "localhost", Port: "5673"},
			want: "amqp://u:p%40ss@localhost:5673/",
		},
	}
	for _, tc := range rmqTests {
		t.Run("RMQ_"+tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cfg.UrlRmq())
		})
	}

	assert.Equal(t, "redis:6379", (RedisConfig{Host: "redis", Port: "6379"}).Addr())
}
