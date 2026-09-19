module github.com/ganfay/split-notify

go 1.26.2

replace github.com/ganfay/split-proto => ../proto

require (
	github.com/ganfay/split-proto v0.0.0-00010101000000-000000000000
	github.com/ilyakaznacheev/cleanenv v1.5.0
	github.com/rabbitmq/amqp091-go v1.12.0
	google.golang.org/grpc v1.82.0
)

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)
