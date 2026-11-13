package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRabbitMQConfigUrlRmqEscaping(t *testing.T) {
	tests := []struct {
		name string
		cfg  RabbitMQConfig
		want string
	}{
		{
			name: "default host and port",
			cfg:  RabbitMQConfig{User: "guest", Password: "secret"},
			want: "amqp://guest:secret@rabbitmq:5672/",
		},
		{
			name: "special characters in credentials",
			cfg: RabbitMQConfig{
				User:     "user@domain",
				Password: "p@ss:1/2#?%",
				Host:     "rmq.internal",
				Port:     "5673",
			},
			want: "amqp://user%40domain:p%40ss%3A1%2F2%23%3F%25@rmq.internal:5673/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.UrlRmq(); got != tc.want {
				t.Fatalf("UrlRmq() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConfigGRpcTarget(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "leading colon port with default host",
			cfg:  Config{GRpcPort: ":50001"},
			want: "bot:50001",
		},
		{
			name: "bare numeric port with custom host",
			cfg:  Config{GRpcHost: "127.0.0.1", GRpcPort: "50001"},
			want: "127.0.0.1:50001",
		},
		{
			name: "ipv6 host",
			cfg:  Config{GRpcHost: "::1", GRpcPort: ":50001"},
			want: "[::1]:50001",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.GRpcTarget(); got != tc.want {
				t.Fatalf("GRpcTarget() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	valid := func() Config {
		return Config{
			Env:      "local",
			GRpcHost: "bot",
			GRpcPort: ":50001",
			RabbitMQ: RabbitMQConfig{
				User:     "guest",
				Password: "guest",
				Host:     "rabbitmq",
				Port:     "5672",
				Queue:    "test",
			},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{name: "valid with colon", mutate: func(c *Config) {}},
		{name: "valid without colon", mutate: func(c *Config) { c.GRpcPort = "50001" }},
		{name: "empty grpc port", mutate: func(c *Config) { c.GRpcPort = "   " }, wantErr: true},
		{name: "double colon grpc port", mutate: func(c *Config) { c.GRpcPort = "::50001" }, wantErr: true},
		{name: "out of range grpc port", mutate: func(c *Config) { c.GRpcPort = ":70000" }, wantErr: true},
		{name: "zero grpc port", mutate: func(c *Config) { c.GRpcPort = ":0" }, wantErr: true},
		{name: "non numeric grpc port", mutate: func(c *Config) { c.GRpcPort = ":abc" }, wantErr: true},
		{name: "signed grpc port", mutate: func(c *Config) { c.GRpcPort = "+50001" }, wantErr: true},
		{name: "empty grpc host", mutate: func(c *Config) { c.GRpcHost = "  " }, wantErr: true},
		{name: "whitespace rmq user", mutate: func(c *Config) { c.RabbitMQ.User = "   " }, wantErr: true},
		{name: "empty rmq pass", mutate: func(c *Config) { c.RabbitMQ.Password = "" }, wantErr: true},
		{name: "empty rmq host", mutate: func(c *Config) { c.RabbitMQ.Host = "" }, wantErr: true},
		{name: "invalid rmq port", mutate: func(c *Config) { c.RabbitMQ.Port = ":5672" }, wantErr: true},
		{name: "empty rmq queue", mutate: func(c *Config) { c.RabbitMQ.Queue = " " }, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}

	var nilCfg *Config
	if err := nilCfg.Validate(); err == nil {
		t.Fatal("Validate() on nil config expected error, got nil")
	}
}

func TestLoadFromFileAndEnv(t *testing.T) {
	t.Run("missing env file falls back to system env", func(t *testing.T) {
		t.Setenv("GRPC_PORT", ":50001")
		t.Setenv("RMQ_USER", "appuser")
		t.Setenv("RMQ_PASS", "apppass")

		missing := filepath.Join(t.TempDir(), "does-not-exist.env")
		cfg, err := LoadFromFile(missing)
		if err != nil {
			t.Fatalf("LoadFromFile() unexpected error: %v", err)
		}
		if cfg.GRpcTarget() != "bot:50001" {
			t.Fatalf("GRpcTarget() = %q, want bot:50001", cfg.GRpcTarget())
		}
		if cfg.RabbitMQ.Queue != "test" {
			t.Fatalf("RabbitMQ.Queue = %q, want test", cfg.RabbitMQ.Queue)
		}
	})

	t.Run("missing required env returns error", func(t *testing.T) {
		t.Setenv("GRPC_PORT", "")
		t.Setenv("RMQ_USER", "")
		t.Setenv("RMQ_PASS", "")
		missing := filepath.Join(t.TempDir(), "missing.env")
		_, err := LoadFromFile(missing)
		if err == nil {
			t.Fatal("expected error for missing required env vars")
		}
	})

	t.Run("valid dotenv file loads and validates", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), ".env")
		content := strings.Join([]string{
			"GRPC_PORT=50002",
			"GRPC_HOST=bot-svc",
			"RMQ_USER=rmquser",
			"RMQ_PASS=rmqpass",
			"RMQ_QUEUE=notifications",
		}, "\n")
		if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		cfg, err := LoadFromFile(envFile)
		if err != nil {
			t.Fatalf("LoadFromFile() unexpected error: %v", err)
		}
		if cfg.GRpcTarget() != "bot-svc:50002" {
			t.Fatalf("GRpcTarget() = %q, want bot-svc:50002", cfg.GRpcTarget())
		}
		if cfg.RabbitMQ.Queue != "notifications" {
			t.Fatalf("RabbitMQ.Queue = %q, want notifications", cfg.RabbitMQ.Queue)
		}
	})

	t.Run("invalid port in env returns validation error", func(t *testing.T) {
		t.Setenv("GRPC_PORT", ":notaport")
		t.Setenv("RMQ_USER", "u")
		t.Setenv("RMQ_PASS", "p")

		_, err := LoadFromFile("")
		if err == nil {
			t.Fatal("expected validation error for invalid GRPC_PORT")
		}
	})
}
